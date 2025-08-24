"""增强版JavBus爬虫 - 解决SSL连接问题并提供更强的网络稳定性"""
import re
import ssl
import time
import random
import logging
import requests
from urllib3.util.retry import Retry
from requests.adapters import HTTPAdapter
from urllib3 import disable_warnings
from urllib3.exceptions import InsecureRequestWarning

from javsp.web.base import *
from javsp.web.exceptions import *
from javsp.func import *
from javsp.config import Cfg, CrawlerID
from javsp.datatype import MovieInfo, GenreMap


# 禁用SSL警告
disable_warnings(InsecureRequestWarning)

logger = logging.getLogger(__name__)

# 配置失败日志记录器
failed_logger = logging.getLogger('javbus2_failed')
#os.makedirs('logs', exist_ok=True)
failed_handler = logging.FileHandler('logs/failed.log', encoding='utf-8')
failed_formatter = logging.Formatter('%(asctime)s - %(message)s')
failed_handler.setFormatter(failed_formatter)
failed_logger.addHandler(failed_handler)
failed_logger.setLevel(logging.INFO)
failed_logger.propagate = False  # 防止日志传播到父记录器

genre_map = GenreMap('data/genre_javbus.csv')
permanent_url = 'https://www.javbus.com'

# 多个User-Agent轮换，增强反爬虫能力
USER_AGENTS = [
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36',
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0',
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15',
]

# 直接使用永久URL，不再使用proxy_free功能
base_url = permanent_url


class EnhancedSession:
    """增强的会话类，提供更强的SSL处理和重试机制"""
    
    def __init__(self):
        self.session = requests.Session()
        self._setup_session()
        self._setup_ssl_context()
    
    def _setup_session(self):
        """配置会话的重试策略和适配器"""
        # 配置重试策略
        retry_strategy = Retry(
            total=Cfg().network.retry,
            backoff_factor=1.5,  # 指数退避
            status_forcelist=[429, 500, 502, 503, 504],
            allowed_methods=["HEAD", "GET", "OPTIONS"]
        )
        
        # 配置HTTP适配器
        adapter = HTTPAdapter(
            max_retries=retry_strategy,
            pool_connections=10,
            pool_maxsize=10,
            pool_block=False
        )
        
        self.session.mount("http://", adapter)
        self.session.mount("https://", adapter)
        
        # 设置基础配置
        self.session.timeout = Cfg().network.timeout.total_seconds()
        self.session.proxies = read_proxy()
    
    def _setup_ssl_context(self):
        """配置SSL上下文以处理SSL连接问题"""
        # 创建自定义SSL上下文
        ssl_context = ssl.create_default_context()
        ssl_context.check_hostname = False
        ssl_context.verify_mode = ssl.CERT_NONE
        
        # 设置支持的SSL协议版本
        ssl_context.minimum_version = ssl.TLSVersion.TLSv1_2
        ssl_context.maximum_version = ssl.TLSVersion.TLSv1_3
        
        # 设置加密套件
        ssl_context.set_ciphers('DEFAULT@SECLEVEL=1')
        
        # 应用SSL上下文到会话
        self.session.verify = False
    
    def get_random_headers(self):
        """获取随机User-Agent和其他头部"""
        return {
            'User-Agent': random.choice(USER_AGENTS),
            'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
            'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
            'Accept-Encoding': 'gzip, deflate, br',
            'Connection': 'keep-alive',
            'Upgrade-Insecure-Requests': '1',
            'Sec-Fetch-Dest': 'document',
            'Sec-Fetch-Mode': 'navigate',
            'Sec-Fetch-Site': 'none',
            'Cache-Control': 'max-age=0',
        }
    
    def test_connection(self, url=None):
        """测试网络连接和代理状态"""
        test_url = url or f'{permanent_url}/test'
        try:
            response = self.session.get(
                test_url,
                headers=self.get_random_headers(),
                timeout=5,
                allow_redirects=True
            )
            return True, response.status_code
        except Exception as e:
            return False, str(e)

    def get_with_retry(self, url, max_attempts=None):
        """带重试的GET请求，支持SSL错误处理和连接诊断"""
        if max_attempts is None:
            max_attempts = Cfg().network.retry
            
        last_exception = None
        
        # 第一次失败时测试基础连接
        connection_tested = False
        
        for attempt in range(max_attempts):
            try:
                # 添加随机延迟避免被检测
                if attempt > 0:
                    delay = random.uniform(1.0, 3.0) * (attempt + 1)
                    time.sleep(delay)
                    logger.debug(f'javbus2: 重试请求 ({attempt + 1}/{max_attempts}): {url}')
                
                # 使用随机头部
                headers = self.get_random_headers()
                
                response = self.session.get(
                    url, 
                    headers=headers,
                    timeout=Cfg().network.timeout.total_seconds(),
                    allow_redirects=True
                )
                
                # 检查响应状态
                if response.status_code == 200:
                    return response
                elif response.status_code == 404:
                    # 404错误不需要重试，直接抛出MovieNotFoundError
                    raise MovieNotFoundError(__name__, url.split('/')[-1])
                elif response.status_code in [403, 503]:
                    logger.warning(f'javbus2: 访问被阻止 ({response.status_code}): {url}')
                    if attempt == max_attempts - 1:
                        raise SiteBlocked(f'javbus2: {response.status_code} 访问被阻止: {url}')
                else:
                    response.raise_for_status()
                    
            except (ssl.SSLError, requests.exceptions.SSLError) as e:
                last_exception = e
                logger.debug(f'javbus2: SSL错误 ({attempt + 1}/{max_attempts}): {e}')
                if attempt == max_attempts - 1:
                    raise requests.exceptions.RequestException(f'javbus2: SSL连接失败: {e}')
                    
            except requests.exceptions.RequestException as e:
                last_exception = e
                # 在第一次网络错误时测试基础连接
                if not connection_tested and attempt == 0:
                    connection_tested = True
                    conn_ok, conn_result = self.test_connection()
                    if not conn_ok:
                        logger.warning(f'javbus2: 基础连接测试失败: {conn_result}')
                        if 'proxy' in str(conn_result).lower():
                            logger.warning('javbus2: 可能的代理连接问题，请检查代理设置')
                
                logger.debug(f'javbus2: 网络错误 ({attempt + 1}/{max_attempts}): {e}')
                if attempt == max_attempts - 1:
                    raise
                    
            except Exception as e:
                last_exception = e
                logger.debug(f'javbus2: 未知错误 ({attempt + 1}/{max_attempts}): {e}')
                if attempt == max_attempts - 1:
                    raise
        
        # 如果所有重试都失败，抛出最后一个异常
        if last_exception:
            raise last_exception


def normalize_dvdid(dvdid):
    """标准化番号格式，去除多余的前导零
    例如: RBD-00841 -> RBD-841, MIDE-00443 -> MIDE-443
    """
    if not dvdid:
        return dvdid
    
    # 使用正则表达式匹配番号格式: 字母-数字
    match = re.match(r'^([A-Za-z]+)-(\d+)$', dvdid.strip())
    if match:
        prefix = match.group(1)
        number = match.group(2)
        # 去除前导零，但保留至少一位数字
        normalized_number = str(int(number))
        normalized_dvdid = f"{prefix}-{normalized_number}"
        
        if normalized_dvdid != dvdid:
            logger.debug(f'javbus2: 番号标准化: {dvdid} -> {normalized_dvdid}')
        
        return normalized_dvdid
    
    # 如果不匹配标准格式，返回原番号
    return dvdid

def try_multiple_dvdid_formats(original_dvdid):
    """标准化番号格式为 XXX-三位数字格式
    返回标准化的番号字符串
    """
    if not original_dvdid:
        return original_dvdid
    
    # 使用正则表达式匹配番号格式: 字母-数字
    match = re.match(r'^([A-Za-z]+)-(\d+)$', original_dvdid.strip())
    if match:
        prefix = match.group(1)
        number = match.group(2)
        # 去除前导零，然后补齐到3位数字
        normalized_number = str(int(number)).zfill(3)
        standardized_dvdid = f"{prefix}-{normalized_number}"
        
        if standardized_dvdid != original_dvdid:
            logger.debug(f'javbus2: 番号标准化: {original_dvdid} -> {standardized_dvdid}')
        
        return standardized_dvdid
    
    # 如果不匹配标准格式，返回原番号
    return original_dvdid


# 全局会话实例
enhanced_session = EnhancedSession()


def parse_data(movie: MovieInfo):
    """从网页抓取并解析指定番号的数据
    Args:
        movie (MovieInfo): 要解析的影片信息，解析后的信息直接更新到此变量内
    """
    original_dvdid = movie.dvdid
    standardized_dvdid = try_multiple_dvdid_formats(original_dvdid)
    
    logger.debug(f'javbus2: 开始抓取: {original_dvdid}，标准化格式: {standardized_dvdid}')
    
    # 使用标准化后的番号格式
    url = f'{base_url}/{standardized_dvdid}'
    logger.debug(f'javbus2: 抓取URL: {url}')
    
    try:
        resp = enhanced_session.get_with_retry(url)
        
        # 处理可能的重定向到登录页面
        if resp.history and any('/doc/driver-verify' in r.url for r in resp.history):
            logger.debug('javbus2: 检测到driver验证页面，使用重定向前的响应')
            # 使用第一个重定向前的响应
            resp = resp.history[0]
        
        html = resp2html(resp)
        
        # 检查是否为404页面
        page_title = html.xpath('/html/head/title/text()')
        if page_title and page_title[0].startswith('404 Page Not Found!'):
            logger.debug(f'javbus2: 番号 {standardized_dvdid} 返回404')
            raise MovieNotFoundError(__name__, standardized_dvdid)
        
        # 如果成功找到页面，继续处理
        logger.debug(f'javbus2: 成功找到影片页面，使用番号格式: {standardized_dvdid}')
        actual_dvdid = standardized_dvdid
        
    except MovieNotFoundError as e:
        failed_logger.info(f'影片未找到: {original_dvdid} (标准化: {standardized_dvdid}) - URL: {url} - {str(e)}')
        raise
    except Exception as e:
        error_type = type(e).__name__
        failed_logger.info(f'抓取失败: {original_dvdid} (标准化: {standardized_dvdid}) - URL: {url} - 错误类型: {error_type} - 详情: {str(e)}')
        logger.debug(f'javbus2: 番号 {standardized_dvdid} 出现异常: {e}')
        raise
    
    # 如果成功找到页面，继续处理数据提取
    try:
        # 检查是否找到了影片容器
        container_list = html.xpath("//div[@class='container']")
        if not container_list:
            logger.warning('javbus2: 未找到影片容器，可能页面结构变化')
            raise MovieNotFoundError(__name__, actual_dvdid)
            
        container = container_list[0]
        
        # 提取标题
        title_list = container.xpath("h3/text()")
        if not title_list:
            logger.warning('javbus2: 未找到影片标题')
            raise MovieNotFoundError(__name__, movie.dvdid)
        title = title_list[0]
        
        # 提取封面
        cover_list = container.xpath("//a[@class='bigImage']/img/@src")
        if not cover_list:
            logger.warning('javbus2: 未找到封面图片')
            cover = None
        else:
            cover = cover_list[0]
        
        # 提取预览图片
        preview_pics = container.xpath("//div[@id='sample-waterfall']/a/@href")
        
        # 提取影片信息区域
        info_list = container.xpath("//div[@class='col-md-3 info']")
        if not info_list:
            logger.warning('javbus2: 未找到影片信息区域')
            raise MovieNotFoundError(__name__, movie.dvdid)
        info = info_list[0]
        
        # 提取番号
        dvdid_elements = info.xpath("p/span[text()='識別碼:']")
        if dvdid_elements:
            parsed_dvdid = dvdid_elements[0].getnext().text
            # 如果网页上的番号与我们使用的不同，记录一下，但优先使用我们找到的格式
            if parsed_dvdid != actual_dvdid:
                logger.debug(f'javbus2: 网页显示番号: {parsed_dvdid}，使用的番号: {actual_dvdid}')
        else:
            logger.warning(f'javbus2: 未找到識別碼标签，使用找到的番号: {actual_dvdid}')
        
        # 提取发布日期
        date_elements = info.xpath("p/span[text()='發行日期:']")
        if date_elements:
            publish_date = date_elements[0].tail.strip()
        else:
            publish_date = None
            logger.debug('javbus2: 未找到发布日期')
        
        # 提取时长
        duration_elements = info.xpath("p/span[text()='長度:']")
        if duration_elements:
            duration = duration_elements[0].tail.replace('分鐘', '').strip()
        else:
            duration = None
            logger.debug('javbus2: 未找到影片时长')
        
        # 提取导演
        director_tag = info.xpath("p/span[text()='導演:']")
        if director_tag:
            director_element = director_tag[0].getnext()
            if director_element is not None:
                movie.director = director_element.text.strip()
        
        # 提取制作商
        producer_tag = info.xpath("p/span[text()='製作商:']")
        if producer_tag:
            producer_element = producer_tag[0].getnext()
            if producer_element is not None and producer_element.text:
                movie.producer = producer_element.text.strip()
        
        # 提取发行商
        publisher_tag = info.xpath("p/span[text()='發行商:']")
        if publisher_tag:
            publisher_element = publisher_tag[0].getnext()
            if publisher_element is not None:
                movie.publisher = publisher_element.text.strip()
        
        # 提取系列
        serial_tag = info.xpath("p/span[text()='系列:']")
        if serial_tag:
            serial_element = serial_tag[0].getnext()
            if serial_element is not None:
                movie.serial = serial_element.text
        
        # 提取类别和类别ID
        genre_tags = info.xpath("//span[@class='genre']/label/a")
        genre, genre_id = [], []
        for tag in genre_tags:
            tag_url = tag.get('href', '')
            pre_id = tag_url.split('/')[-1] if tag_url else ''
            genre_text = tag.text
            if genre_text:
                genre.append(genre_text)
                if 'uncensored' in tag_url:
                    movie.uncensored = True
                    genre_id.append('uncensored-' + pre_id)
                else:
                    movie.uncensored = False
                    genre_id.append(pre_id)
        
        # 提取女优信息
        actress, actress_pics = [], {}
        actress_tags = html.xpath("//a[@class='avatar-box']/div/img")
        for tag in actress_tags:
            name = tag.get('title')
            pic_url = tag.get('src')
            if name:
                actress.append(name)
                if pic_url and not pic_url.endswith('nowprinting.gif'):
                    actress_pics[name] = pic_url
        
        # 更新movie对象的属性
        movie.url = f'{permanent_url}/{actual_dvdid}'
        movie.dvdid = actual_dvdid  # 使用实际找到的番号格式
        movie.title = title.replace(actual_dvdid, '').strip()
        movie.cover = cover
        movie.preview_pics = preview_pics
        if publish_date and publish_date != '0000-00-00':
            movie.publish_date = publish_date
        if duration and duration.isdigit() and int(duration) > 0:
            movie.duration = duration
        movie.genre = genre
        movie.genre_id = genre_id
        movie.actress = actress
        movie.actress_pics = actress_pics
        
        logger.debug(f'javbus2: 成功抓取影片信息: {actual_dvdid} (原始: {original_dvdid})')
        
    except (MovieNotFoundError, SiteBlocked):
        raise
    except Exception as e:
        error_type = type(e).__name__
        failed_logger.info(f'数据解析失败: {original_dvdid} (标准化: {actual_dvdid}) - URL: {url} - 错误类型: {error_type} - 详情: {str(e)}')
        logger.error(f'javbus2: 抓取数据时发生异常: {e}', exc_info=True)
        raise WebsiteError(f'javbus2: 抓取数据失败: {e}')


def parse_clean_data(movie: MovieInfo):
    """解析指定番号的影片数据并进行清洗"""
    try:
        parse_data(movie)
        if movie.genre_id:
            movie.genre_norm = genre_map.map(movie.genre_id)
            movie.genre_id = None  # 清空genre id，表明已完成转换
        logger.info(f'javbus2: 数据抓取和清洗完成: {movie.dvdid}')
    except Exception as e:
        error_type = type(e).__name__
        failed_logger.info(f'数据清洗失败: {movie.dvdid} - 错误类型: {error_type} - 详情: {str(e)}')
        logger.error(f'javbus2: 数据清洗失败: {e}')
        raise


if __name__ == "__main__":
    import pretty_errors
    pretty_errors.configure(display_link=True)
    
    # 配置日志级别
    logging.basicConfig(level=logging.DEBUG)
    
    # 测试用例 - 测试番号标准化功能
    movie = MovieInfo('RBD-00841')
    try:
        print(f"开始测试javbus2爬虫，测试番号: {movie.dvdid}")
        parse_clean_data(movie)
        print("测试成功！抓取到的数据：")
        print(movie)
    except CrawlerError as e:
        print(f"爬虫错误: {e}")
        logger.error(e, exc_info=1)
    except Exception as e:
        print(f"未知错误: {e}")
        logger.error(e, exc_info=1)