-- IM系统数据库初始化脚本
-- init.sql

-- 创建数据库
CREATE DATABASE IF NOT EXISTS im_db DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE im_db;

-- 用户表
CREATE TABLE IF NOT EXISTS users (
    user_id VARCHAR(64) PRIMARY KEY,
    username VARCHAR(64) NOT NULL UNIQUE,
    nickname VARCHAR(64),
    avatar VARCHAR(512),
    password_hash VARCHAR(256) NOT NULL,
    status TINYINT DEFAULT 1 COMMENT '1-正常 0-禁用',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 群组表
CREATE TABLE IF NOT EXISTS groups (
    group_id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    avatar VARCHAR(512),
    announcement TEXT,
    description VARCHAR(512),
    owner_id VARCHAR(64) NOT NULL,
    max_members INT DEFAULT 500,
    member_count INT DEFAULT 0,
    mute_all BOOLEAN DEFAULT FALSE,
    join_mode TINYINT DEFAULT 0 COMMENT '0-自由加入 1-需审批 2-禁止加入',
    status TINYINT DEFAULT 1 COMMENT '1-正常 0-已解散',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_owner (owner_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 群成员表
CREATE TABLE IF NOT EXISTS group_members (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    group_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    role TINYINT DEFAULT 0 COMMENT '0-成员 1-管理员 2-群主',
    nickname VARCHAR(64),
    mute_until BIGINT DEFAULT 0,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    inviter_id VARCHAR(64),
    UNIQUE INDEX idx_group_user (group_id, user_id),
    INDEX idx_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 消息记录表模板（按月分表）
-- 可以使用存储过程自动创建分表
CREATE TABLE IF NOT EXISTS messages (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    message_id VARCHAR(64) NOT NULL UNIQUE,
    conversation_id VARCHAR(128) NOT NULL,
    type TINYINT NOT NULL COMMENT '1-单聊 2-群聊 3-系统',
    from_user_id VARCHAR(64) NOT NULL,
    to_id VARCHAR(64) NOT NULL,
    content TEXT,
    seq BIGINT NOT NULL,
    revoked BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_conversation_seq (conversation_id, seq),
    INDEX idx_from_user (from_user_id),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 消息分表示例（2024年1月）
CREATE TABLE IF NOT EXISTS messages_202401 LIKE messages;
CREATE TABLE IF NOT EXISTS messages_202402 LIKE messages;
CREATE TABLE IF NOT EXISTS messages_202403 LIKE messages;
CREATE TABLE IF NOT EXISTS messages_202404 LIKE messages;
CREATE TABLE IF NOT EXISTS messages_202405 LIKE messages;
CREATE TABLE IF NOT EXISTS messages_202406 LIKE messages;
CREATE TABLE IF NOT EXISTS messages_202407 LIKE messages;
CREATE TABLE IF NOT EXISTS messages_202408 LIKE messages;
CREATE TABLE IF NOT EXISTS messages_202409 LIKE messages;
CREATE TABLE IF NOT EXISTS messages_202410 LIKE messages;
CREATE TABLE IF NOT EXISTS messages_202411 LIKE messages;
CREATE TABLE IF NOT EXISTS messages_202412 LIKE messages;

-- 离线消息表
CREATE TABLE IF NOT EXISTS offline_messages (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    message_id VARCHAR(64) NOT NULL UNIQUE,
    conversation_id VARCHAR(128) NOT NULL,
    content TEXT NOT NULL,
    pushed BOOLEAN DEFAULT FALSE,
    pushed_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expire_at TIMESTAMP NOT NULL,
    INDEX idx_user_created (user_id, created_at),
    INDEX idx_expire (expire_at),
    INDEX idx_pushed (pushed)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 设备表（用于推送）
CREATE TABLE IF NOT EXISTS devices (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    device_token VARCHAR(256) NOT NULL UNIQUE,
    platform VARCHAR(16) NOT NULL COMMENT 'ios/android/web',
    app_version VARCHAR(32),
    device_info VARCHAR(256) COMMENT '设备信息',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 会话表
CREATE TABLE IF NOT EXISTS conversations (
    conversation_id VARCHAR(128) PRIMARY KEY,
    type TINYINT NOT NULL COMMENT '1-单聊 2-群聊',
    last_message_id VARCHAR(64),
    last_message_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 用户会话关系表
CREATE TABLE IF NOT EXISTS user_conversations (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    conversation_id VARCHAR(128) NOT NULL,
    unread_count INT DEFAULT 0,
    last_read_seq BIGINT DEFAULT 0,
    muted BOOLEAN DEFAULT FALSE,
    pinned BOOLEAN DEFAULT FALSE,
    deleted BOOLEAN DEFAULT FALSE,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_user_conv (user_id, conversation_id),
    INDEX idx_user_updated (user_id, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 文件记录表
CREATE TABLE IF NOT EXISTS files (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    file_id VARCHAR(64) NOT NULL UNIQUE,
    user_id VARCHAR(64) NOT NULL,
    file_name VARCHAR(256) NOT NULL,
    file_size BIGINT NOT NULL,
    file_ext VARCHAR(32),
    mime_type VARCHAR(128),
    file_type TINYINT COMMENT '1-图片 2-视频 3-音频 4-文档 5-压缩包 6-其他',
    storage_path VARCHAR(512) NOT NULL,
    thumbnail_path VARCHAR(512),
    md5 VARCHAR(64),
    width INT DEFAULT 0,
    height INT DEFAULT 0,
    duration INT DEFAULT 0 COMMENT '音视频时长(秒)',
    status TINYINT DEFAULT 1 COMMENT '1-正常 0-已删除',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user (user_id),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 消息已读状态表
CREATE TABLE IF NOT EXISTS message_read_status (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    conversation_id VARCHAR(128) NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    last_read_seq BIGINT DEFAULT 0,
    last_read_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_conv_user (conversation_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 好友关系表
CREATE TABLE IF NOT EXISTS friendships (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    friend_id VARCHAR(64) NOT NULL,
    remark VARCHAR(64) COMMENT '好友备注',
    status TINYINT DEFAULT 1 COMMENT '1-正常 0-已删除 2-拉黑',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_user_friend (user_id, friend_id),
    INDEX idx_friend (friend_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 好友申请表
CREATE TABLE IF NOT EXISTS friend_requests (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    from_user_id VARCHAR(64) NOT NULL,
    to_user_id VARCHAR(64) NOT NULL,
    message VARCHAR(256) COMMENT '申请留言',
    status TINYINT DEFAULT 0 COMMENT '0-待处理 1-已同意 2-已拒绝 3-已忽略',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_to_user (to_user_id, status),
    INDEX idx_from_user (from_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 群组加入申请表
CREATE TABLE IF NOT EXISTS group_join_requests (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    group_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    message VARCHAR(256) COMMENT '申请留言',
    status TINYINT DEFAULT 0 COMMENT '0-待处理 1-已同意 2-已拒绝',
    handler_id VARCHAR(64) COMMENT '处理人',
    handled_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_group_status (group_id, status),
    INDEX idx_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 系统配置表
CREATE TABLE IF NOT EXISTS system_configs (
    id INT AUTO_INCREMENT PRIMARY KEY,
    config_key VARCHAR(64) NOT NULL UNIQUE,
    config_value TEXT,
    description VARCHAR(256),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 插入默认配置
INSERT INTO system_configs (config_key, config_value, description) VALUES
('max_group_members', '500', '群组最大成员数'),
('max_offline_messages', '1000', '用户最大离线消息数'),
('offline_message_expire_days', '7', '离线消息过期天数'),
('max_file_size', '104857600', '单个文件最大大小(字节)'),
('max_image_size', '10485760', '图片最大大小(字节)'),
('max_video_size', '104857600', '视频最大大小(字节)')
ON DUPLICATE KEY UPDATE config_value = VALUES(config_value);

-- 创建存储过程：自动创建消息分表
DELIMITER //

CREATE PROCEDURE IF NOT EXISTS create_message_table(IN table_suffix VARCHAR(6))
BEGIN
    SET @sql = CONCAT('CREATE TABLE IF NOT EXISTS messages_', table_suffix, ' LIKE messages');
    PREPARE stmt FROM @sql;
    EXECUTE stmt;
    DEALLOCATE PREPARE stmt;
END //

-- 创建存储过程：清理过期离线消息
CREATE PROCEDURE IF NOT EXISTS clean_expired_offline_messages()
BEGIN
    DELETE FROM offline_messages WHERE expire_at < NOW();
END //

-- 创建存储过程：更新群成员数
CREATE PROCEDURE IF NOT EXISTS update_group_member_count(IN p_group_id VARCHAR(64))
BEGIN
    UPDATE groups
    SET member_count = (SELECT COUNT(*) FROM group_members WHERE group_id = p_group_id)
    WHERE group_id = p_group_id;
END //

DELIMITER ;

-- 创建事件：每天清理过期离线消息
CREATE EVENT IF NOT EXISTS evt_clean_offline_messages
ON SCHEDULE EVERY 1 DAY
STARTS CURRENT_TIMESTAMP
DO CALL clean_expired_offline_messages();
