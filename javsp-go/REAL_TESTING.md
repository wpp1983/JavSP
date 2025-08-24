# 真实测试指南 (Real Testing Guide)

这个文档介绍了JavSP-Go项目中新增的真实网站测试功能。

## 概述

除了现有的单元测试和集成测试（使用Mock数据），我们新增了以下真实测试类型：

1. **真实网站爬虫测试** - 对真实的JavBus和AVWiki网站进行测试
2. **真实图片下载测试** - 测试从互联网下载真实图片
3. **网络容错测试** - 测试各种网络故障场景的处理

## 测试命令

### 基本命令

```bash
# 运行所有真实网站测试（需要网络连接）
make test-real

# 运行网络容错测试
make test-network

# 运行包含真实测试在内的所有测试
make test-all

# 运行除真实网站测试外的所有测试（适合CI环境）
make test-ci-safe
```

### Go命令

```bash
# 直接使用go test运行真实测试
go test -v -tags="integration && real_sites" ./test/integration/...

# 只运行爬虫测试
go test -v -tags="integration && real_sites" -run="TestReal.*Crawler" ./test/integration/...

# 只运行下载测试
go test -v -tags="integration && real_sites" -run="TestReal.*Download" ./test/integration/...

# 只运行网络容错测试
go test -v -tags="integration && real_sites" -run="TestNetwork.*" ./test/integration/...
```

## 测试类型详解

### 1. 真实爬虫测试 (`real_crawler_test.go`)

**测试内容：**
- 验证JavBus和AVWiki爬虫的可用性
- 使用已知存在的电影ID测试数据提取
- 验证HTML解析逻辑在真实页面上的表现
- 检查数据完整性和格式正确性

**测试的电影ID：**
```go
// JavBus测试ID
"STARS-123", "SSIS-001", "IPX-177", "PRED-456"

// AVWiki测试ID  
"SSIS-698", "STARS-256", "IPX-001"
```

**验证项目：**
- 基本字段提取（标题、发布日期、女优、类型等）
- URL格式验证
- HTML标签清理
- 数据格式标准化

### 2. 真实下载测试 (`real_downloader_test.go`)

**测试内容：**
- 下载不同格式的测试图片（JPEG、PNG、WebP）
- 测试并发下载功能
- 验证进度回调机制
- 测试错误处理（404、服务器错误等）
- 测试大文件下载
- 验证下载统计功能

**测试URL：**
- `https://httpbin.org/image/jpeg` - 小JPEG图片
- `https://httpbin.org/image/png` - 小PNG图片
- `https://httpbin.org/image/webp` - 小WebP图片

**验证项目：**
- 文件成功下载并保存
- 文件大小和内容类型正确
- 错误情况下的清理工作
- 并发下载的稳定性

### 3. 网络容错测试 (`network_resilience_test.go`)

**测试场景：**

#### DNS故障
- 使用不存在的域名测试DNS解析失败
- 验证错误处理和超时机制

#### 连接超时
- 测试连接到不响应的端口
- 验证超时设置的有效性

#### 服务器错误
- 测试各种HTTP错误码（500、502、503、504）
- 验证重试机制

#### 慢速服务器
- 测试服务器响应缓慢的情况
- 验证超时和取消机制

#### 网络恢复
- 测试间歇性故障的恢复能力
- 验证重试后成功的场景

## 配置选项

### 环境变量

```bash
# 设置代理测试URL（可选）
export TEST_PROXY_URL="http://proxy.example.com:8080"

# 设置测试超时（可选）
export TEST_TIMEOUT="300s"
```

### 测试配置

真实测试使用了以下配置以确保对目标网站的友好访问：

```go
// 爬虫配置
config := &crawler.CrawlerConfig{
    Timeout:    30 * time.Second,
    MaxRetries: 3,
    RetryDelay: 2 * time.Second,
    RateLimit:  3 * time.Second, // 请求间隔3秒
    UserAgent:  "Mozilla/5.0 (compatible; JavSP-Test/1.0)",
}

// 下载器配置
config := downloader.DefaultDownloadConfig()
config.MaxConcurrency = 3        // 最大并发数
config.Timeout = 30 * time.Second
config.MaxFileSize = 10 * 1024 * 1024 // 10MB限制
```

## 注意事项

### ⚠️ 重要警告

1. **网络依赖**: 真实测试需要稳定的网络连接
2. **外部依赖**: 测试依赖外部网站的可用性
3. **执行时间**: 真实测试比Mock测试耗时更长
4. **频率限制**: 避免频繁运行以免被目标网站限制

### 🚫 不适合的场景

- **CI/CD流水线**: 使用`make test-ci-safe`代替
- **离线环境**: 无网络连接时跳过
- **快速开发**: 开发时优先使用单元测试

### ✅ 适合的场景

- **发布前验证**: 确保在真实环境中的功能正常
- **网站变更检测**: 及时发现目标网站结构变化
- **性能基准测试**: 在真实网络条件下的性能表现
- **问题排查**: 复现用户报告的网络相关问题

## 故障排除

### 常见问题

#### 1. DNS解析失败
```
Error: no such host
```
**解决方案**: 检查网络连接和DNS设置

#### 2. 连接超时
```
Error: dial tcp: i/o timeout
```
**解决方案**: 检查防火墙设置或增加超时时间

#### 3. 403/429错误
```
Error: HTTP 403/429
```
**解决方案**: 降低测试频率，等待后重试

#### 4. 证书错误
```
Error: x509: certificate verify failed
```
**解决方案**: 网络环境可能有证书问题，检查代理设置

### 调试技巧

1. **启用详细日志**:
```bash
go test -v -tags="integration && real_sites" ./test/integration/... -args -test.v
```

2. **单独运行测试**:
```bash
go test -v -tags="integration && real_sites" -run="TestRealJavBusCrawler" ./test/integration/...
```

3. **设置更长超时**:
```bash
go test -timeout=600s -tags="integration && real_sites" ./test/integration/...
```

## 开发指南

### 添加新的真实测试

1. **选择测试类型**: 确定是爬虫、下载还是网络容错测试
2. **添加构建标签**: 使用`//go:build integration && real_sites`
3. **设置合理超时**: 真实测试需要更长的超时时间
4. **添加错误处理**: 网络测试可能失败，需要优雅处理
5. **记录测试日志**: 便于调试和监控

### 测试数据选择

**爬虫测试**:
- 选择知名、稳定存在的电影ID
- 避免使用可能被删除的新电影
- 定期验证测试数据的有效性

**下载测试**:
- 使用稳定的测试服务（如httpbin.org）
- 选择小文件以减少测试时间
- 避免使用可能变化的真实网站图片

## 监控和维护

### 定期检查

1. **每月运行**: `make test-real` 检查网站兼容性
2. **更新测试数据**: 替换失效的测试电影ID
3. **监控执行时间**: 检查网络性能变化
4. **审查日志**: 查找潜在的兼容性问题

### 测试结果分析

```bash
# 运行测试并保存详细日志
make test-real 2>&1 | tee real-test-results.log

# 分析成功率
grep -c "PASS\|FAIL" real-test-results.log

# 查看错误模式  
grep "Error\|Failed" real-test-results.log
```

---

**最后更新**: 2024年12月
**维护者**: JavSP团队