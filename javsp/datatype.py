"""定义数据类型和一些通用性的对数据类型的操作"""
import os
import csv
import json
import shutil
import logging
from functools import cached_property

from javsp.config import Cfg
from javsp.lib import resource_path, detect_special_attr, is_folder_safe_to_delete


logger = logging.getLogger(__name__)
filemove_logger = logging.getLogger('filemove')

class MovieInfo:
    def __init__(self, dvdid: str = None, /, *, cid: str = None, from_file=None):
        """
        Args:
            dvdid ([str], optional): 番号，要通过其他方式创建实例时此参数应留空
            from_file: 从指定的文件(json格式)中加载数据来创建实例
        """
        arg_count = len([i for i in [dvdid, cid, from_file] if i])
        if arg_count != 1:
            raise TypeError(f'Require 1 parameter but {arg_count} given')
        if isinstance(dvdid, Movie):
            self.dvdid = dvdid.dvdid
            self.cid = dvdid.cid
        else:
            self.dvdid = dvdid      # DVD ID，即通常的番号
            self.cid = cid          # DMM Content ID
        # 创建类的默认属性
        self.url = None             # 影片页面的URL
        self.plot = None            # 故事情节
        self.cover = None           # 封面图片（URL）
        self.big_cover = None       # 高清封面图片（URL）
        self.genre = None           # 影片分类的标签
        self.genre_id = None        # 影片分类的标签的ID，用于解决部分站点多个genre同名的问题，也便于管理多语言的genre
        self.genre_norm = None      # 统一后的影片分类的标签
        self.score = None           # 评分（10分制，为方便提取写入和保持统一，应以字符串类型表示）
        self.title = None           # 影片标题（不含番号）
        self.ori_title = None       # 原始影片标题，仅在标题被处理过时才对此字段赋值
        self.magnet = None          # 磁力链接
        self.serial = None          # 系列
        self.actress = None         # 出演女优
        self.actress_pics = None    # 出演女优的头像。单列一个字段，便于满足不同的使用需要
        self.director = None        # 导演
        self.duration = None        # 影片时长
        self.producer = None        # 制作商
        self.publisher = None       # 发行商
        self.uncensored = None      # 是否为无码影片
        self.publish_date = None    # 发布日期
        self.preview_pics = None    # 预览图片（URL）
        self.preview_video = None   # 预览视频（URL）

        if from_file:
            if os.path.isfile(from_file):
                self.load(from_file)
            else:
                raise TypeError(f"Invalid file path: '{from_file}'")

    def __str__(self) -> str:
        d = vars(self)
        return json.dumps(d, indent=2, ensure_ascii=False)

    def __repr__(self) -> str:
        if self.dvdid:
            expression = f"('{self.dvdid}')"
        else:
            expression = f"('cid={self.cid}')"
        return __class__.__name__ + expression

    def __eq__(self, other) -> bool:
        if isinstance(other, self.__class__):
            return self.__dict__ == other.__dict__
        else:
            return False

    def dump(self, filepath=None, crawler=None) -> None:
        if not filepath:
            id = self.dvdid if self.dvdid else self.cid
            if crawler:
                filepath = f'../unittest/data/{id} ({crawler}).json'
                filepath = os.path.join(os.path.dirname(__file__), filepath)
            else:
                filepath = id + '.json'
        with open(filepath, 'wt', encoding='utf-8') as f:
            f.write(str(self))

    def load(self, filepath) -> None:
        with open(filepath, 'rt', encoding='utf-8') as f:
            d = json.load(f)
        # 更新对象属性
        attrs = vars(self).keys()
        for k, v in d.items():
            if k in attrs:
                self.__setattr__(k, v)

    def get_info_dic(self):
        """生成用来填充模板的字典"""
        info = self
        d = {}
        d['num'] = info.dvdid or info.cid
        d['title'] = info.title or Cfg().summarizer.default.title
        d['rawtitle'] = info.ori_title or d['title']
        d['actress'] = ','.join(info.actress) if info.actress else Cfg().summarizer.default.actress
        d['score'] = info.score or '0'
        d['censor'] = Cfg().summarizer.censor_options_representation[1 if info.uncensored else 0]
        d['serial'] = info.serial or Cfg().summarizer.default.series
        d['director'] = info.director or Cfg().summarizer.default.director
        d['producer'] = info.producer or Cfg().summarizer.default.producer
        d['publisher'] = info.publisher or Cfg().summarizer.default.publisher
        d['date'] = info.publish_date or '0000-00-00'
        d['year'] = d['date'].split('-')[0]
        # cid中不会出现'-'，可以直接从d['num']拆分出label
        num_items = d['num'].split('-')
        d['label'] = num_items[0] if len(num_items) > 1 else '---'
        d['genre'] = ','.join(info.genre_norm if info.genre_norm else info.genre if info.genre else [])

        return d


class Movie:
    """用于关联影片文件的类"""
    def __init__(self, dvdid=None, /, *, cid=None) -> None:
        arg_count = len([i for i in (dvdid, cid) if i])
        if arg_count != 1:
            raise TypeError(f'Require 1 parameter but {arg_count} given')
        # 创建类的默认属性
        self.dvdid = dvdid              # DVD ID，即通常的番号
        self.cid = cid                  # DMM Content ID
        self.files = []                 # 关联到此番号的所有影片文件的列表（用于管理带有多个分片的影片）
        self.data_src = 'normal'        # 数据源：不同的数据源将使用不同的爬虫
        self.info: MovieInfo = None     # 抓取到的影片信息
        self.save_dir = None            # 存放影片、封面、NFO的文件夹路径
        self.basename = None            # 按照命名模板生成的不包含路径和扩展名的basename
        self.nfo_file = None            # nfo文件的路径
        self.fanart_file = None         # fanart文件的路径
        self.poster_file = None         # poster文件的路径
        self.guid = None                # GUI使用的唯一标识，通过dvdid和files做md5生成

    @cached_property
    def hard_sub(self) -> bool:
        """影片文件带有内嵌字幕"""
        return 'C' in self.attr_str

    @cached_property
    def uncensored(self) -> bool:
        """影片文件是无码流出/无码破解版本（很多种子并不严格区分这两种，故这里也不进一步细分）"""
        return 'U' in self.attr_str

    @cached_property
    def attr_str(self) -> str:
        """用来标示影片文件的额外属性的字符串(空字符串/-U/-C/-UC)"""
        # 暂不支持多分片的影片
        if len(self.files) != 1:
            return ''
        r = detect_special_attr(self.files[0], self.dvdid)
        if r:
            r = '-' + r
        return r

    def __repr__(self) -> str:
        if self.cid and self.data_src == 'cid':
            expression = f"('cid={self.cid}')"
        else:
            expression = f"('{self.dvdid}')"
        return __class__.__name__ + expression

    def rename_files(self, use_hardlink: bool = False) -> None:
        """根据命名规则移动（重命名）影片文件"""
        def move_file(src:str, dst:str) -> bool:
            """移动（重命名）文件并记录信息到日志
            
            Returns:
                bool: True if file was moved, False if source was deleted due to duplicate
            """
            abs_dst = os.path.abspath(dst)
            abs_src = os.path.abspath(src)
            
            # 如果目标文件已存在，删除源文件（当前正在处理的文件）
            if os.path.exists(abs_dst):
                logger.info(f'目标文件已存在，删除当前文件: {os.path.relpath(src)}')
                try:
                    os.remove(abs_src)
                    logger.debug(f'已删除重复的源文件: {abs_src}')
                    return False  # 源文件被删除，没有进行移动
                except OSError as e:
                    logger.error(f'删除重复文件失败: {abs_src}, 错误: {e}')
                    raise FileExistsError(f'目标文件已存在且无法删除源文件: {abs_dst}')
            
            # 确保目标目录存在
            os.makedirs(os.path.dirname(abs_dst), exist_ok=True)
            
            if (use_hardlink):
                os.link(src, abs_dst)
            else:
                shutil.move(src, abs_dst)
            src_rel = os.path.relpath(src)
            dst_name = os.path.basename(dst)
            logger.info(f"重命名文件: '{src_rel}' -> '...{os.sep}{dst_name}'")
            # 目前StreamHandler并未设置filter，为了避免显示中出现重复的日志，这里暂时只能用debug级别
            filemove_logger.debug(f'移动（重命名）文件: \n  原路径: "{src}"\n  新路径: "{abs_dst}"')
            return True  # 文件成功移动

        new_paths = []
        dir = os.path.dirname(self.files[0])
        if len(self.files) == 1:
            fullpath = self.files[0]
            ext = os.path.splitext(fullpath)[1]
            newpath = os.path.join(self.save_dir, self.basename + ext)
            if move_file(fullpath, newpath):
                new_paths.append(newpath)
        else:
            for i, fullpath in enumerate(self.files, start=1):
                ext = os.path.splitext(fullpath)[1]
                newpath = os.path.join(self.save_dir, self.basename + f'-CD{i}' + ext)
                if move_file(fullpath, newpath):
                    new_paths.append(newpath)
        self.new_paths = new_paths
        
        # 根据配置决定是否进行文件夹清理
        if Cfg().summarizer.path.cleanup_empty_folders:
            self._cleanup_source_folder(dir)
    
    def _cleanup_source_folder(self, folder_path: str) -> None:
        """清理源文件夹，如果只包含可删除的文件（图片、元数据等）则删除整个文件夹"""
        if not os.path.exists(folder_path) or not os.path.isdir(folder_path):
            return
        
        try:
            # 使用配置中的视频扩展名
            video_extensions = Cfg().scanner.filename_extensions
            can_delete, important_files = is_folder_safe_to_delete(folder_path, video_extensions)
            
            if can_delete:
                # 删除文件夹中的所有文件
                for item in os.listdir(folder_path):
                    item_path = os.path.join(folder_path, item)
                    try:
                        if os.path.isfile(item_path):
                            os.remove(item_path)
                            logger.debug(f'删除文件: {item}')
                        elif os.path.isdir(item_path):
                            # 递归删除子目录（如果为空）
                            try:
                                os.rmdir(item_path)
                                logger.debug(f'删除空子目录: {item}')
                            except OSError:
                                logger.debug(f'子目录非空，跳过: {item}')
                    except OSError as e:
                        logger.warning(f'删除文件失败: {item}, 错误: {e}')
                
                # 删除空文件夹
                try:
                    os.rmdir(folder_path)
                    logger.info(f'已清理源文件夹: {folder_path}')
                except OSError as e:
                    logger.debug(f'删除源文件夹失败: {folder_path}, 错误: {e}')
            else:
                if important_files:
                    logger.debug(f'源文件夹包含重要文件，跳过清理: {folder_path}')
                    logger.debug(f'重要文件列表: {important_files}')
                else:
                    logger.debug(f'源文件夹为空，删除: {folder_path}')
                    try:
                        os.rmdir(folder_path)
                        logger.info(f'已删除空文件夹: {folder_path}')
                    except OSError:
                        pass
                        
        except Exception as e:
            logger.debug(f'文件夹清理时出错: {folder_path}, 错误: {e}')


class GenreMap(dict):
    """genre的映射表"""
    def __init__(self, file):
        genres = {}
        with open(resource_path(file), newline='', encoding='utf-8-sig') as csvfile:
            reader = csv.DictReader(csvfile)
            try:
                for row in reader:
                    genres[row['id']] = row['translate']
            except UnicodeDecodeError:
                logger.error('CSV file must be saved as UTF-8-BOM to edit is in Excel')
            except KeyError:
                logger.error("The columns 'id' and 'translate' must exist in the csv file")
        self.update(genres)

    def map(self, ls):
        """将列表ls按照内置的映射进行替换：保留映射表中不存在的键，删除值为空的键"""
        mapped = [self.get(i, i) for i in ls]
        cleaned = [i for i in mapped if i]  # 译文为空表示此genre应当被删除
        return cleaned
