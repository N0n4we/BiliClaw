这是一个B站（Bilibili）多线程爬虫系统，采用生产者-消费者流水线架构，配合 Kafka 消息队列和 Flink ETL
实现实时数据处理。

---
整体架构

搜索视频 → 获取视频详情 → 爬取评论 → 爬取用户信息
    ↓           ↓            ↓            ↓
Kafka Topics (claw_video/claw_comment/claw_account)
                            ↓
                    Flink ETL 处理
                            ↓
                    ClickHouse 数据仓库

---
核心组件

1. crawler.py - 爬虫引擎（4阶段流水线）
┌─────────┬────────────────────────────────────┬────────────────────┐
│  阶段   │                功能                │        输出        │
├─────────┼────────────────────────────────────┼────────────────────┤
│ Stage 1 │ 按关键词搜索视频，多线程并行分页   │ 视频BVID列表       │
├─────────┼────────────────────────────────────┼────────────────────┤
│ Stage 2 │ 获取视频详情，提取UP主MID          │ 视频元数据 → Kafka │
├─────────┼────────────────────────────────────┼────────────────────┤
│ Stage 3 │ 爬取一级评论和二级回复（游标分页） │ 评论数据 → Kafka   │
├─────────┼────────────────────────────────────┼────────────────────┤
│ Stage 4 │ 根据收集的MID爬取用户资料          │ 账号数据 → Kafka   │
└─────────┴────────────────────────────────────┴────────────────────┘
断点续爬机制：通过 video_comment_progress.json
保存评论爬取进度（游标位置），中断后可从上次位置继续。

2. api.py - B站API封装（反反爬）

- WBI签名：实现B站的 w_rid 签名算法，动态获取 img_key 和 sub_key
- Cookie池轮换：多账号Cookie轮流使用，避免单账号被限流
- 令牌桶限流：默认2 QPS，防止触发风控
- 指数退避重试：网络错误自动重试，最多3次

3. storage.py - 数据持久化

- 发送数据到 Kafka 对应 topic
- 维护已发送记录（sent_records/ 目录），避免重复
- 保存/加载断点进度

4. cookie_pool.py - Cookie管理

- 支持轮询/随机策略
- 自动标记失效Cookie（连续失败3次禁用）
- 线程安全

---
数据流向

spider/main.py (入口配置)
       ↓
BiliCrawler.run()
       ↓
┌──────────────────────────────────────────┐
│  search_worker → video_worker →          │
│  comment_worker → account_worker         │
└──────────────────────────────────────────┘
       ↓
Kafka: claw_video / claw_comment / claw_account
       ↓
Flink SQL ETL (etl/*.flink.sql)
       ↓
ClickHouse 表 (dim_video, dim_comment, dim_account)

---
关键设计

1. 队列解耦：各阶段通过 queue.Queue 通信，互不阻塞
2. 线程同步：使用 threading.Event 协调流水线完成信号
3. 去重机制：本地文件记录已处理的 BVID/RPID/MID
4. 容错恢复：resume=True 时跳过已爬取内容，从断点继续

这套架构实现了高并发、可恢复、实时入库的B站数据采集系统。
