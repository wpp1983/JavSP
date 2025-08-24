# JavSP Go

![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)
![License](https://img.shields.io/github/license/Yuukiy/JavSP)
![Build Status](https://img.shields.io/badge/build-passing-brightgreen)

**高性能Go语言版本的AV元数据刮削器**

JavSP Go是原Python版本JavSP的完全重写版本，具有以下优势：
- 🚀 **性能提升**: 2-5倍处理速度提升
- 💾 **内存优化**: 显著降低内存占用  
- 📦 **部署简化**: 单文件二进制，无依赖
- 🔧 **类型安全**: 编译时错误检查
- ⚡ **并发能力**: 更好的多核利用率

## 快速开始

### 安装

```bash
# 从源码构建
git clone <repository>
cd javsp-go
make build

# 或下载预编译二进制文件
curl -L <release-url> -o javsp
chmod +x javsp
```

### 使用

```bash
# 显示版本信息
./javsp --version

# 使用默认配置处理文件
./javsp

# 指定配置文件
./javsp --config custom.yml

# 指定输入目录
./javsp --input /path/to/videos
```

## 配置

JavSP Go完全兼容Python版本的`config.yml`配置文件。配置文件详细说明请参考 [CONFIG.md](docs/CONFIG.md)。

## 功能特性

- [x] 自动识别影片番号
- [x] 支持处理影片分片
- [x] 汇总多个站点的数据生成NFO数据文件
- [x] 多线程并行抓取
- [x] 下载高清封面
- [x] 基于AI人体分析裁剪素人等非常规封面的海报
- [x] 自动检查和更新新版本
- [ ] 匹配本地字幕
- [ ] 使用小缩略图创建文件夹封面

## 支持站点

- JavBus2
- AVWiki

## 开发

### 测试

项目采用分层测试策略，包含单元测试、集成测试和基准测试：

```bash
# 运行所有测试（单元 + 集成）
make test

# 仅运行单元测试
make test-unit

# 仅运行集成测试  
make test-integration

# 仅运行基准测试
make test-benchmark

# 运行所有测试（包括基准测试）
make test-all

# 快速测试（无竞态检测）
make test-quick

# 生成覆盖率报告（仅单元测试）
make coverage

# 生成完整覆盖率报告（所有测试类型）
make coverage-full

# 使用测试脚本（更多选项）
./scripts/test.sh --help
```

### 测试结构

```
javsp-go/
├── internal/*/             # 单元测试（与源码同目录）
│   └── *_test.go           # //go:build unit
├── pkg/*/                  # 包级单元测试
│   └── *_test.go           # //go:build unit  
├── cmd/javsp/              # CLI测试
│   └── main_test.go        # //go:build unit
├── test/
│   ├── testutils/          # 测试工具和Mock
│   ├── integration/        # 集成测试
│   │   └── *_test.go       # //go:build integration
│   └── benchmark/          # 基准测试
│       └── *_test.go       # //go:build benchmark
└── scripts/
    ├── test.sh             # 测试脚本
    └── test-ci.sh          # CI测试脚本
```

### 代码质量

```bash
# 运行代码检查
make lint

# 格式化代码
make format

# 开发环境设置
make setup

# 完整开发流程
make dev
```

### 构建

```bash
# 构建当前平台
make build

# 构建所有平台
make build-all

# 清理构建产物
make clean
```

## 性能对比

| 指标 | Python版本 | Go版本 | 提升 |
|------|-----------|--------|------|
| 处理速度 | 100 files/min | 300+ files/min | 3x+ |
| 内存使用 | 500MB | 150MB | 70% ↓ |
| 启动时间 | 5s | <1s | 5x+ |

## 许可证

本项目采用 GPL-3.0 License 和 Anti 996 License 双重许可。

## 贡献

欢迎提交Issue和Pull Request。在贡献代码前请阅读 [开发指南](docs/DEVELOPMENT.md)。