-- 仅检查，不改配置/表/索引，不执行 EXPLAIN ANALYZE。
-- 使用只读角色；在受控运维会话执行。先确认目标数据库，禁止复制凭据到记录。
BEGIN READ ONLY;
SET LOCAL statement_timeout = '5s';
SET LOCAL lock_timeout = '1s';
SELECT version(), current_database(), current_setting('TimeZone');
SELECT pg_size_pretty(pg_relation_size('usage_logs')) AS heap,
       pg_size_pretty(pg_indexes_size('usage_logs')) AS indexes,
       pg_size_pretty(pg_total_relation_size('usage_logs')) AS total;
SELECT n_live_tup,n_dead_tup,n_mod_since_analyze,last_analyze,last_autoanalyze,
       last_vacuum,last_autovacuum,seq_scan,seq_tup_read,idx_scan
FROM pg_stat_user_tables WHERE relname='usage_logs';
SELECT indexname,indexdef FROM pg_indexes WHERE tablename='usage_logs';
SELECT attname,null_frac,n_distinct,correlation
FROM pg_stats WHERE tablename='usage_logs' AND attname IN ('created_at','model','group_id','account_id');
SELECT heap_blks_read,heap_blks_hit,idx_blks_read,idx_blks_hit,
       round(100.0*heap_blks_hit/NULLIF(heap_blks_hit+heap_blks_read,0),2) AS lifetime_heap_hit_pct
FROM pg_statio_user_tables WHERE relname='usage_logs';
SELECT datname,blks_read,blks_hit,temp_files,temp_bytes,blk_read_time,blk_write_time,stats_reset
FROM pg_stat_database WHERE datname=current_database();
-- 不输出查询全文，避免把其他用户请求/敏感文本带入文档。
SELECT state,wait_event_type,wait_event,count(*),max(clock_timestamp()-query_start) AS longest
FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid()
GROUP BY state,wait_event_type,wait_event;
SELECT name,setting,unit FROM pg_settings WHERE name IN
 ('shared_buffers','effective_cache_size','work_mem','max_parallel_workers_per_gather','track_io_timing');
COMMIT;
-- 以上累积计数至少间隔 60 秒采样两次求差值，不能把生命周期命中率当事故窗口命中率。
-- 增长率需要历史监控/多次采样；n_dead_tup 是估计，不等于已证实膨胀。
-- 若部署为分区表，须另查 pg_partition_tree('usage_logs') 的叶表统计和索引。
-- PostgreSQL 16+ 可另行读取 pg_stat_io；pg_stat_statements 仅在已安装时读取，勿临时建扩展。
-- EXPLAIN (FORMAT JSON, COSTS TRUE, SETTINGS TRUE) <原 countSQL / 原 resultsSQL / 新 SQL>;
-- 将实际请求的 UTC start/end、platform/group、LIMIT/OFFSET 显式代入，并在同一时间边界比较。
-- 必须保留三份计划，尤其关注 Seq/Bitmap/Index Scan、估算行数、连接和聚合节点。
-- 不以新旧 SQL 文本代替真实执行计划，不在高负载生产跑 30d ANALYZE。
-- 主机侧：受控采集 iostat -dx 1 10（或等价工具）、磁盘规格/吞吐额度，关联 wait_event。
-- 不在生产清 OS/PG 缓存、不执行 VACUUM FULL、REINDEX、CREATE INDEX 或统计信息刷新。
