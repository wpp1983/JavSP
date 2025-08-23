"""用来组织不需要依赖任何自定义类型的功能函数"""
import os
import re
import sys
from pathlib import Path


__all__ = ['re_escape', 'resource_path', 'strftime_to_minutes', 'detect_special_attr', 'truncate_filename', 'is_folder_safe_to_delete']


_special_chars_map = {i: '\\' + chr(i) for i in b'()[]{}?*+|^$\\.'}
def re_escape(s: str) -> str:
    """用来对字符串进行转义，以将转义后的字符串用于构造正则表达式"""
    pattern = s.translate(_special_chars_map)
    return pattern


def resource_path(path: str) -> str:
    """获取一个随代码打包的文件在解压后的路径"""
    if getattr(sys, "frozen", False):
        return path
    else:
        path_joined = Path(__file__).parent.parent / path
        return str(path_joined)


def strftime_to_minutes(s: str) -> int:
    """将HH:MM:SS或MM:SS的时长转换为分钟数返回

    Args:
        s (str): HH:MM:SS or MM:SS

    Returns:
        [int]: 取整后的分钟数
    """
    items = list(map(int, s.split(':')))
    if len(items) == 2:
        minutes = items[0] + round(items[1]/60)
    elif len(items) == 3:
        minutes = items[0] * 60 + items[1] + round(items[2]/60)
    else:
        raise ValueError(f"无法将字符串'{s}'转换为分钟")
    return minutes


_PATTERN = re.compile(r'(uncen(sor(ed)?)?([- _\s]*leak(ed)?)?|[无無][码碼](流出|破解))', flags=re.I)
def detect_special_attr(filepath: str, avid: str = None) -> str:
    """通过文件名检测影片是否有特殊属性（内嵌字幕、无码流出/破解）

    Returns:
        [str]: '', 'U', 'C', 'UC'
    """
    result = ''
    base = os.path.splitext(os.path.basename(filepath))[0].upper()
    # 尝试使用正则匹配
    match = _PATTERN.search(base)
    if match:
        result += 'U'
    # 尝试匹配-C/-U/-UC后缀的影片
    postfix = base.split('-')[-1]
    if postfix in ('U', 'C', 'UC'):
        result += postfix
    elif avid:
        pattern_str = re.sub(r'[_-]', '[_-]*', avid) + r'(UC|U|C)\b'
        match = re.search(pattern_str, base, flags=re.I)
        if match:
            result += match.group(1)
    # 最终格式化
    result = ''.join(sorted(set(result), reverse=True))
    return result


def truncate_filename(filename: str, max_length: int, by_byte: bool = False, preserve_ext: bool = True) -> str:
    """智能截断文件名以符合长度限制
    
    Args:
        filename: 原始文件名
        max_length: 最大长度限制
        by_byte: 是否按字节计算长度
        preserve_ext: 是否保留扩展名
        
    Returns:
        截断后的文件名
    """
    if not filename:
        return filename
        
    # 分离扩展名
    base_name, ext = os.path.splitext(filename) if preserve_ext else (filename, '')
    
    # 计算长度的函数
    def get_length(s: str) -> int:
        return len(s.encode('utf-8')) if by_byte else len(s)
    
    # 如果总长度已经符合要求，直接返回
    if get_length(filename) <= max_length:
        return filename
    
    # 预留扩展名的长度
    ext_length = get_length(ext)
    available_length = max_length - ext_length
    
    if available_length <= 0:
        # 如果扩展名本身就超长，只保留部分扩展名
        if preserve_ext and ext_length > 0:
            truncated_ext = ext[:max_length] if not by_byte else ext.encode('utf-8')[:max_length].decode('utf-8', errors='ignore')
            return truncated_ext
        return ""
    
    # 优先在标点符号处截断，保持语义完整性
    punctuation_chars = '。！？；，、：""''（）【】《》〈〉…·～—-_=+|\\/*&^%$#@'
    
    # 先按标点符号分割
    parts = []
    current_part = ""
    
    for char in base_name:
        current_part += char
        if char in punctuation_chars:
            parts.append(current_part)
            current_part = ""
    
    if current_part:
        parts.append(current_part)
    
    # 从后往前移除部分，直到长度符合要求
    result = ""
    for part in parts:
        test_result = result + part
        if get_length(test_result) <= available_length:
            result = test_result
        else:
            # 如果添加当前部分会超长，尝试只添加部分内容
            remaining_length = available_length - get_length(result)
            if remaining_length > 0:
                if by_byte:
                    # 按字节截断
                    part_bytes = part.encode('utf-8')
                    truncated_bytes = part_bytes[:remaining_length]
                    # 确保不在多字节字符中间截断
                    while truncated_bytes:
                        try:
                            truncated_bytes.decode('utf-8')
                            break
                        except UnicodeDecodeError:
                            truncated_bytes = truncated_bytes[:-1]
                    result += truncated_bytes.decode('utf-8', errors='ignore')
                else:
                    # 按字符截断
                    result += part[:remaining_length]
            break
    
    # 如果仍然为空或者截断结果不够理想，使用简单截断
    if not result or get_length(result) > available_length:
        if by_byte:
            result_bytes = base_name.encode('utf-8')[:available_length]
            # 确保不在多字节字符中间截断
            while result_bytes:
                try:
                    result_bytes.decode('utf-8')
                    break
                except UnicodeDecodeError:
                    result_bytes = result_bytes[:-1]
            result = result_bytes.decode('utf-8', errors='ignore')
        else:
            result = base_name[:available_length]
    
    return result + ext


def is_folder_safe_to_delete(folder_path: str, keep_video_extensions: list = None) -> tuple[bool, list[str]]:
    """判断文件夹是否可以安全删除（只包含图片、元数据等可删除文件）
    
    Args:
        folder_path: 要检查的文件夹路径
        keep_video_extensions: 需要保留的视频文件扩展名列表，如果为None则使用默认列表
        
    Returns:
        tuple[bool, list[str]]: (是否可以安全删除, 重要文件列表)
    """
    if not os.path.exists(folder_path) or not os.path.isdir(folder_path):
        return True, []
    
    # 默认的视频文件扩展名（需要保留的重要文件）
    if keep_video_extensions is None:
        keep_video_extensions = [
            '.3gp', '.avi', '.f4v', '.flv', '.iso', '.m2ts', '.m4v', '.mkv', 
            '.mov', '.mp4', '.mpeg', '.rm', '.rmvb', '.ts', '.vob', '.webm', 
            '.wmv', '.strm', '.mpg'
        ]
    
    # 可以删除的文件类型（图片、元数据、字幕等）
    deletable_extensions = {
        # 图片文件
        '.jpg', '.jpeg', '.png', '.gif', '.bmp', '.tiff', '.tif', '.webp',
        # 元数据和信息文件
        '.nfo', '.xml', '.txt', '.json', '.db', '.ini', '.log',
        # 字幕文件
        '.srt', '.ass', '.ssa', '.vtt', '.sub', '.idx', '.sup',
        # 其他常见的非重要文件
        '.url', '.lnk', '.torrent', '.magnet', '.md5', '.sha1',
        # 临时文件
        '.tmp', '.temp', '.bak', '.old', '.~', '.swp',
        # 系统文件
        '.ds_store', 'thumbs.db', 'desktop.ini',
    }
    
    # 可以删除的特殊文件名（不区分大小写）
    deletable_filenames = {
        'thumbs.db', 'desktop.ini', '.ds_store', 'folder.jpg', 'folder.png',
        'cover.jpg', 'cover.png', 'poster.jpg', 'poster.png', 'fanart.jpg',
        'fanart.png', 'background.jpg', 'background.png', 'banner.jpg',
        'banner.png', 'logo.jpg', 'logo.png', 'movie.nfo', 'tvshow.nfo',
        'season.nfo', 'episode.nfo'
    }
    
    important_files = []
    
    try:
        for item in os.listdir(folder_path):
            item_path = os.path.join(folder_path, item)
            
            # 跳过子目录（暂不处理嵌套目录）
            if os.path.isdir(item_path):
                important_files.append(f"子目录: {item}")
                continue
            
            item_lower = item.lower()
            _, ext = os.path.splitext(item_lower)
            
            # 检查是否是需要保留的视频文件
            if ext in [e.lower() for e in keep_video_extensions]:
                important_files.append(f"视频文件: {item}")
                continue
            
            # 检查是否是可删除的文件类型
            if ext in deletable_extensions:
                continue
                
            # 检查是否是可删除的特殊文件名
            if item_lower in deletable_filenames:
                continue
            
            # 检查文件大小，如果很小可能是元数据文件
            try:
                file_size = os.path.getsize(item_path)
                if file_size < 1024 * 10:  # 小于10KB的文件可能是元数据
                    continue
            except OSError:
                pass
            
            # 其他未识别的文件视为重要文件
            important_files.append(f"未知文件: {item}")
    
    except OSError as e:
        return False, [f"访问目录失败: {e}"]
    
    # 如果没有重要文件，则可以安全删除
    return len(important_files) == 0, important_files


if __name__ == "__main__":
    print(detect_special_attr('ipx-177cd1.mp4', 'IPX-177'))
    # 测试文件名截断功能
    test_cases = [
        ("短文件名.mp4", 50, False),
        ("这是一个非常长的文件名，包含了很多中文字符，应该会被截断.mp4", 50, False),
        ("very_long_filename_with_english_characters_that_should_be_truncated.mp4", 50, False),
        ("混合中英文的超长文件名with_english_and_中文_characters测试.mp4", 60, True),
    ]
    
    for filename, max_len, by_byte in test_cases:
        truncated = truncate_filename(filename, max_len, by_byte)
        print(f"原文件名: {filename}")
        print(f"截断后: {truncated}")
        print(f"长度: {len(truncated.encode('utf-8')) if by_byte else len(truncated)}/{max_len}")
        print("---")
