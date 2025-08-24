"""AV-Wiki.net 爬虫 - 提供额外的影片信息来源"""
#import os
import re
import logging
import requests
from urllib.parse import urljoin

from javsp.web.base import *
from javsp.web.exceptions import *
from javsp.func import *
from javsp.config import Cfg
from javsp.datatype import MovieInfo


logger = logging.getLogger(__name__)

# 配置失败日志记录器
failed_logger = logging.getLogger('avwiki_failed')
#os.makedirs('logs', exist_ok=True)
failed_handler = logging.FileHandler('logs/failed.log', encoding='utf-8')
failed_formatter = logging.Formatter('%(asctime)s - %(message)s')
failed_handler.setFormatter(failed_formatter)
failed_logger.addHandler(failed_handler)
failed_logger.setLevel(logging.INFO)
failed_logger.propagate = False  # 防止日志传播到父记录器

permanent_url = 'https://av-wiki.net'


def parse_data(movie: MovieInfo):
    """从AV-Wiki网页抓取并解析指定番号的数据
    Args:
        movie (MovieInfo): 要解析的影片信息，解析后的信息直接更新到此变量内
    """
    dvdid = movie.dvdid
    logger.debug(f'avwiki: 开始抓取: {dvdid}')
    
    # 构建请求URL
    url = f'{permanent_url}/{dvdid}'
    logger.debug(f'avwiki: 抓取URL: {url}')
    
    try:
        resp = request_get(url, delay_raise=True)
        
        # 先检查HTTP状态码
        if resp.status_code == 404:
            logger.debug(f'avwiki: 番号 {dvdid} 未找到（HTTP 404，正常现象）')
            raise MovieNotFoundError(__name__, dvdid)
        
        # 对于其他HTTP错误，正常抛出
        resp.raise_for_status()
        
        html = resp2html(resp)
        
        # 检查是否找到页面
        page_title = html.xpath('//title/text()')
        if page_title and ('404' in page_title[0] or 'Not Found' in page_title[0]):
            logger.debug(f'avwiki: 番号 {dvdid} 未找到（正常现象）')
            # 404 是正常现象，不记录到失败日志
            raise MovieNotFoundError(__name__, dvdid)
        
        # 提取标题 - 从h1标签获取
        title_elements = html.xpath('//h1/text()')
        if title_elements:
            title = title_elements[0]
            # 清理标题，去掉"に出てるAV女優名まとめ"等后缀
            title = re.sub(r'に出てる.*$', '', title)
            title = re.sub(r'まとめ$', '', title)
            movie.title = title.strip()
        
        # 提取封面图片
        cover_elements = html.xpath('//img[contains(@src, "dmm.co.jp") or contains(@src, "mgstage.com")]/@src')
        if cover_elements:
            movie.cover = cover_elements[0]
        
        # 提取演员信息 - 从页面内容中查找
        content_text = ' '.join(html.xpath('//div[@class="entry-content"]//text()'))
        
        # 查找演员名单
        actress_list = []
        # 从页面标题中提取演员名 (格式: "演员A与演员B与演员C")
        if movie.title:
            # 处理日文中的"と"连接符
            actors_in_title = re.split(r'[とと、,]', movie.title)
            for actor in actors_in_title:
                actor = actor.strip()
                # 过滤掉明显不是人名的部分
                if len(actor) >= 2 and not any(x in actor for x in ['AV', '女優', 'まとめ', 'LUXU', 'MIUM', 'GANA', 'SIRO', 'ABP', 'SSIS']):
                    actress_list.append(actor)
        
        if actress_list:
            movie.actress = actress_list
        
        # 从页面内容中提取制作信息
        if content_text:
            # 提取制作商信息
            producer_match = re.search(r'メーカー[：:]\s*([^\\n]+)', content_text)
            if producer_match:
                movie.producer = producer_match.group(1).strip()
            
            # 提取发布日期
            date_match = re.search(r'配信開始日[：:]?\s*(\d{4}-\d{1,2}-\d{1,2})', content_text)
            if date_match:
                movie.publish_date = date_match.group(1)
            
            # 提取系列信息
            series_match = re.search(r'シリーズ[：:]\s*([^\\n]+)', content_text)
            if series_match:
                movie.serial = series_match.group(1).strip()
        
        # 设置URL和其他基础信息
        movie.url = url
        movie.uncensored = False  # AV-Wiki主要是有码内容
        
        logger.debug(f'avwiki: 成功抓取影片信息: {dvdid}')
        
    except MovieNotFoundError as e:
        raise
    except requests.exceptions.HTTPError as e:
        if e.response.status_code == 404:
            logger.debug(f'avwiki: 番号 {dvdid} 未找到（HTTP 404，正常现象）')
            raise MovieNotFoundError(__name__, dvdid)
        else:
            error_type = type(e).__name__
            failed_logger.info(f'抓取失败: {dvdid} - URL: {url} - 错误类型: {error_type} - 详情: {str(e)}')
            logger.error(f'avwiki: 抓取数据时发生异常: {e}', exc_info=True)
            raise WebsiteError(f'avwiki: 抓取数据失败: {e}')
    except Exception as e:
        error_type = type(e).__name__
        failed_logger.info(f'抓取失败: {dvdid} - URL: {url} - 错误类型: {error_type} - 详情: {str(e)}')
        logger.error(f'avwiki: 抓取数据时发生异常: {e}', exc_info=True)
        raise WebsiteError(f'avwiki: 抓取数据失败: {e}')


def parse_clean_data(movie: MovieInfo):
    """解析指定番号的影片数据并进行清洗"""
    try:
        parse_data(movie)
        logger.info(f'avwiki: 数据抓取和清洗完成: {movie.dvdid}')
    except Exception as e:
        error_type = type(e).__name__
        failed_logger.info(f'数据清洗失败: {movie.dvdid} - 错误类型: {error_type} - 详情: {str(e)}')
        logger.error(f'avwiki: 数据清洗失败: {e}')
        raise


if __name__ == "__main__":
    import pretty_errors
    pretty_errors.configure(display_link=True)
    
    # 配置日志级别
    logging.basicConfig(level=logging.DEBUG)
    
    # 测试用例
    movie = MovieInfo('SSIS-698')
    try:
        print(f"开始测试avwiki爬虫，测试番号: {movie.dvdid}")
        parse_clean_data(movie)
        print("测试成功！抓取到的数据：")
        print(movie)
    except CrawlerError as e:
        print(f"爬虫错误: {e}")
        logger.error(e, exc_info=1)
    except Exception as e:
        print(f"未知错误: {e}")
        logger.error(e, exc_info=1)