CREATE TABLE IF NOT EXISTS biliclaw.dim_account
(
    mid UInt64 COMMENT '用户UID',
    mid_str String COMMENT '用户UID字符串',
    name String COMMENT '用户昵称',
    sex String COMMENT '用户性别',
    sign String COMMENT '用户签名',
    face String COMMENT '用户头像URL',
    `rank` String COMMENT '用户等级排名',
    level UInt8 COMMENT '用户等级',
    birthday String COMMENT '用户生日',
    vip_type UInt8 COMMENT 'VIP类型(0=无,1=月度,2=年度)',
    vip_status UInt8 COMMENT 'VIP状态',
    vip_due_date UInt64 COMMENT 'VIP到期时间戳',
    vip_label_text String COMMENT 'VIP标签文本',
    vip_theme_type UInt8 COMMENT 'VIP主题类型',
    vip_nickname_color String COMMENT '昵称颜色',
    vip_role UInt8 COMMENT 'VIP角色',
    official_role UInt8 COMMENT '认证角色',
    official_title String COMMENT '认证标题',
    official_desc String COMMENT '认证描述',
    official_type Int8 COMMENT '认证类型(-1=无)',
    pendant_pid UInt32 COMMENT '头像框ID',
    pendant_name String COMMENT '头像框名称',
    nameplate_nid UInt32 COMMENT '勋章ID',
    nameplate_name String COMMENT '勋章名称',
    nameplate_level String COMMENT '勋章等级',
    nameplate_condition String COMMENT '勋章条件',
    follower UInt64 COMMENT '粉丝数',
    following UInt32 COMMENT '关注数',
    archive_count UInt32 COMMENT '视频数',
    article_count UInt32 COMMENT '专栏数',
    like_num UInt64 COMMENT '获赞数',
    is_senior_member UInt8 COMMENT '是否硬核会员',
    etl_time DateTime DEFAULT now() COMMENT 'ETL处理时间'
)
ENGINE = ReplacingMergeTree(etl_time)
ORDER BY mid
SETTINGS index_granularity = 8192
COMMENT 'B站账户维度表';
