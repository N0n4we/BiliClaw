CREATE TABLE IF NOT EXISTS biliclaw.bilibili_comment_wide
(
    -- ==================== 评论基础信息 ====================
    rpid UInt64 COMMENT '评论ID',
    rpid_str String COMMENT '评论ID字符串',
    oid UInt64 COMMENT '目标ID(视频aid)',
    oid_str String COMMENT '目标ID字符串',
    comment_type UInt8 COMMENT '评论类型(1=视频)',

    -- 评论层级关系
    root_id UInt64 COMMENT '根评论ID(0=一级评论)',
    root_id_str String COMMENT '根评论ID字符串',
    parent_id UInt64 COMMENT '父评论ID(0=一级评论)',
    parent_id_str String COMMENT '父评论ID字符串',
    dialog_id UInt64 COMMENT '对话ID',
    dialog_id_str String COMMENT '对话ID字符串',
    comment_level UInt8 COMMENT '评论层级(1=一级,2=二级)',

    -- 评论统计
    reply_count UInt32 COMMENT '回复总数',
    reply_count_display UInt32 COMMENT '显示的回复数',
    like_count UInt32 COMMENT '点赞数',

    -- 评论状态
    state Int8 COMMENT '评论状态',
    attr UInt32 COMMENT '评论属性',
    fansgrade UInt8 COMMENT '粉丝等级',
    action UInt8 COMMENT '当前用户操作状态',
    is_up_liked UInt8 COMMENT 'UP主是否点赞',
    is_up_replied UInt8 COMMENT 'UP主是否回复',

    -- 评论时间
    comment_ctime UInt32 COMMENT '评论创建时间戳',
    comment_ctime_dt DateTime COMMENT '评论创建时间',
    comment_time_desc String COMMENT '评论时间描述',

    -- 评论内容
    content_message String COMMENT '评论内容',
    content_message_length UInt32 COMMENT '评论内容长度',
    content_max_line UInt8 COMMENT '最大显示行数',
    has_jump_url UInt8 COMMENT '是否包含跳转链接',
    jump_url_count UInt8 COMMENT '跳转链接数量',
    at_user_count UInt8 COMMENT '@用户数量',

    -- 评论控制
    sub_reply_entry_text String COMMENT '子回复入口文本',
    sub_reply_title_text String COMMENT '子回复标题文本',
    has_folded UInt8 COMMENT '是否有折叠',
    is_folded UInt8 COMMENT '是否被折叠',
    is_invisible UInt8 COMMENT '是否不可见',
    support_share UInt8 COMMENT '是否支持分享',

    -- ==================== 评论者信息 ====================
    commenter_mid UInt64 COMMENT '评论者UID',
    commenter_mid_str String COMMENT '评论者UID字符串',
    commenter_name String COMMENT '评论者昵称',
    commenter_sex String COMMENT '评论者性别',
    commenter_sign String COMMENT '评论者签名',
    commenter_avatar String COMMENT '评论者头像URL',
    commenter_rank String COMMENT '评论者等级排名',
    commenter_level UInt8 COMMENT '评论者等级',
    commenter_is_senior_member UInt8 COMMENT '是否硬核会员',

    -- 评论者VIP信息
    commenter_vip_type UInt8 COMMENT '评论者VIP类型(0=无,1=月度,2=年度)',
    commenter_vip_status UInt8 COMMENT '评论者VIP状态',
    commenter_vip_due_date UInt64 COMMENT '评论者VIP到期时间戳',
    commenter_vip_label String COMMENT '评论者VIP标签文本',
    commenter_vip_theme_type UInt8 COMMENT '评论者VIP主题类型',
    commenter_nickname_color String COMMENT '评论者昵称颜色',

    -- 评论者认证信息
    commenter_official_type Int8 COMMENT '评论者认证类型(-1=无)',
    commenter_official_desc String COMMENT '评论者认证描述',

    -- 评论者装扮信息
    commenter_pendant_id UInt32 COMMENT '评论者头像框ID',
    commenter_pendant_name String COMMENT '评论者头像框名称',
    commenter_nameplate_id UInt32 COMMENT '评论者勋章ID',
    commenter_nameplate_name String COMMENT '评论者勋章名称',
    commenter_nameplate_level String COMMENT '评论者勋章等级',

    -- 评论者粉丝装扮
    commenter_sailing_cardbg_id UInt32 COMMENT '评论者卡片背景ID',
    commenter_sailing_cardbg_name String COMMENT '评论者卡片背景名称',
    commenter_sailing_fan_is_fan UInt8 COMMENT '是否是粉丝',
    commenter_sailing_fan_number UInt32 COMMENT '粉丝编号',
    commenter_sailing_fan_name String COMMENT '粉丝团名称',

    -- 评论者其他属性
    commenter_is_contractor UInt8 COMMENT '是否是承包者',
    commenter_contract_desc String COMMENT '承包描述',

    -- 评论者统计信息(来自dim_account)
    commenter_follower_count UInt64 COMMENT '评论者粉丝数',
    commenter_following_count UInt32 COMMENT '评论者关注数',
    commenter_archive_count UInt32 COMMENT '评论者视频数',
    commenter_article_count UInt32 COMMENT '评论者专栏数',
    commenter_like_num UInt64 COMMENT '评论者获赞数',

    -- ==================== 被回复者信息(二级评论) ====================
    parent_reply_member_mid UInt64 COMMENT '被回复者UID',
    parent_reply_member_name String COMMENT '被回复者昵称',

    -- ==================== 视频信息 ====================
    video_bvid String COMMENT '视频BVID',
    video_aid UInt64 COMMENT '视频AID',
    video_title String COMMENT '视频标题',
    video_title_length UInt16 COMMENT '视频标题长度',
    video_desc String COMMENT '视频简介',
    video_desc_length UInt32 COMMENT '视频简介长度',
    video_pic String COMMENT '视频封面URL',
    video_duration UInt32 COMMENT '视频时长(秒)',
    video_videos_count UInt16 COMMENT '视频分P数',

    -- 视频分区
    video_tid UInt32 COMMENT '视频分区ID',
    video_tid_v2 UInt32 COMMENT '视频分区ID v2',
    video_tname String COMMENT '视频分区名称',
    video_tname_v2 String COMMENT '视频分区名称 v2',

    -- 视频时间
    video_pubdate UInt32 COMMENT '视频发布时间戳',
    video_pubdate_dt DateTime COMMENT '视频发布时间',
    video_ctime UInt32 COMMENT '视频创建时间戳',
    video_ctime_dt DateTime COMMENT '视频创建时间',

    -- 视频属性
    video_copyright UInt8 COMMENT '版权类型(1=自制,2=转载)',
    video_state Int16 COMMENT '视频状态',
    video_is_cooperation UInt8 COMMENT '是否联合投稿',
    video_is_story UInt8 COMMENT '是否Story模式',
    video_is_upower_exclusive UInt8 COMMENT '是否充电专属',

    -- 视频权限
    video_no_reprint UInt8 COMMENT '禁止转载',
    video_autoplay UInt8 COMMENT '自动播放',
    video_download UInt8 COMMENT '允许下载',

    -- 视频统计
    video_view_count UInt64 COMMENT '视频播放量',
    video_danmaku_count UInt32 COMMENT '视频弹幕数',
    video_reply_count UInt32 COMMENT '视频评论数',
    video_favorite_count UInt32 COMMENT '视频收藏数',
    video_coin_count UInt32 COMMENT '视频投币数',
    video_share_count UInt32 COMMENT '视频分享数',
    video_like_count UInt32 COMMENT '视频点赞数',
    video_his_rank UInt32 COMMENT '历史最高排名',
    video_now_rank UInt32 COMMENT '当前排名',

    -- 视频合集信息
    video_season_id UInt64 COMMENT '合集ID',
    video_season_title String COMMENT '合集标题',
    video_season_ep_count UInt32 COMMENT '合集视频数',
    video_mission_id UInt64 COMMENT '活动ID',

    -- 视频尺寸
    video_dimension_width UInt16 COMMENT '视频宽度',
    video_dimension_height UInt16 COMMENT '视频高度',
    video_dimension_rotate UInt8 COMMENT '视频旋转',

    -- ==================== UP主信息 ====================
    up_mid UInt64 COMMENT 'UP主UID',
    up_name String COMMENT 'UP主昵称',
    up_face String COMMENT 'UP主头像URL',

    -- ==================== UP主详细信息(来自accounts) ====================
    up_sex String COMMENT 'UP主性别',
    up_sign String COMMENT 'UP主签名',
    up_level UInt8 COMMENT 'UP主等级',
    up_rank String COMMENT 'UP主排名',

    -- UP主VIP信息
    up_vip_type UInt8 COMMENT 'UP主VIP类型',
    up_vip_status UInt8 COMMENT 'UP主VIP状态',
    up_vip_due_date UInt64 COMMENT 'UP主VIP到期时间',

    -- UP主认证信息
    up_official_role UInt8 COMMENT 'UP主认证角色',
    up_official_title String COMMENT 'UP主认证标题',
    up_official_type Int8 COMMENT 'UP主认证类型',

    -- UP主装扮
    up_pendant_id UInt32 COMMENT 'UP主头像框ID',
    up_pendant_name String COMMENT 'UP主头像框名称',
    up_nameplate_id UInt32 COMMENT 'UP主勋章ID',
    up_nameplate_name String COMMENT 'UP主勋章名称',
    up_nameplate_level String COMMENT 'UP主勋章等级',

    -- UP主统计
    up_follower_count UInt64 COMMENT 'UP主粉丝数',
    up_following_count UInt32 COMMENT 'UP主关注数',
    up_archive_count UInt32 COMMENT 'UP主视频数',
    up_article_count UInt32 COMMENT 'UP主专栏数',
    up_like_num UInt64 COMMENT 'UP主获赞数',

    -- UP主其他属性
    up_is_senior_member UInt8 COMMENT 'UP主是否硬核会员',

    -- ==================== 衍生计算字段 ====================
    -- 时间衍生
    comment_date Date COMMENT '评论日期',
    comment_hour UInt8 COMMENT '评论小时',
    comment_day_of_week UInt8 COMMENT '评论星期几',
    video_publish_date Date COMMENT '视频发布日期',

    -- 时间差计算
    comment_video_interval_days Int32 COMMENT '评论距视频发布天数',
    comment_video_interval_hours Int64 COMMENT '评论距视频发布小时数',

    -- 互动率计算辅助
    is_top_level_comment UInt8 COMMENT '是否一级评论',
    has_replies UInt8 COMMENT '是否有回复',

    -- 用户关系
    is_commenter_up UInt8 COMMENT '评论者是否是UP主',
    is_commenter_vip UInt8 COMMENT '评论者是否VIP',
    is_up_vip UInt8 COMMENT 'UP主是否VIP',

    -- ==================== 元数据 ====================
    data_source String DEFAULT 'bilibili_spider' COMMENT '数据来源',
    etl_time DateTime DEFAULT now() COMMENT 'ETL处理时间',
    data_version String DEFAULT '1.0' COMMENT '数据版本'
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(comment_ctime_dt)
ORDER BY (video_bvid, rpid, comment_ctime)
SETTINGS index_granularity = 8192
COMMENT 'B站评论大宽表 - 以评论为粒度，包含评论、评论者、视频、UP主全维度信息';
