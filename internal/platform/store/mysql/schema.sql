CREATE TABLE IF NOT EXISTS users (
  id VARCHAR(64) NOT NULL,
  email VARCHAR(255) NOT NULL,
  display_name VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_users_email (email)
);

CREATE TABLE IF NOT EXISTS workspaces (
  id VARCHAR(64) NOT NULL,
  name VARCHAR(255) NOT NULL,
  artifact_root VARCHAR(1024) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS workspace_members (
  workspace_id VARCHAR(64) NOT NULL,
  user_id VARCHAR(64) NOT NULL,
  role VARCHAR(32) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (workspace_id, user_id)
);

CREATE TABLE IF NOT EXISTS audit_events (
  id VARCHAR(64) NOT NULL,
  workspace_id VARCHAR(64) NOT NULL,
  actor_user_id VARCHAR(64) NOT NULL,
  action VARCHAR(64) NOT NULL,
  subject_type VARCHAR(64) NOT NULL,
  subject_id VARCHAR(255) NOT NULL DEFAULT '',
  metadata_json MEDIUMTEXT NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_audit_workspace (workspace_id, created_at)
);

CREATE TABLE IF NOT EXISTS demands (
  id VARCHAR(64) NOT NULL,
  workspace_id VARCHAR(64) NOT NULL,
  demand_key VARCHAR(255) NOT NULL,
  title VARCHAR(512) NOT NULL DEFAULT '',
  state VARCHAR(64) NOT NULL DEFAULT '',
  attention VARCHAR(255) NOT NULL DEFAULT '',
  artifact_path VARCHAR(1024) NOT NULL DEFAULT '',
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_demands_ws_key (workspace_id, demand_key)
);