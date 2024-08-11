-- Gochan master template for new database script
-- Contains macros in the form [curlybrace open]macro text[curlybrace close]
-- Macros are substituted by build_initdb.py to the supported database files. Must not contain extra spaces
-- Versioning numbering goes by whole numbers. Upgrade script migrate existing databases between versions
-- Foreign and unique constraints must be named so they can be dropped. 
-- MySQL requires constraint names to be unique globally, hence the long constraint names.
-- Database version: 1

CREATE TABLE DBPREFIXdatabase_version(
	component VARCHAR(40) NOT NULL PRIMARY KEY,
	version INT NOT NULL
);

CREATE TABLE DBPREFIXsections(
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	abbreviation TEXT NOT NULL,
	position SMALLINT NOT NULL,
	hidden BOOL NOT NULL
);

CREATE TABLE DBPREFIXboards(
	id BIGSERIAL PRIMARY KEY,
	section_id BIGINT NOT NULL,
	uri VARCHAR(45) NOT NULL,
	dir VARCHAR(45) NOT NULL,
	navbar_position SMALLINT NOT NULL,
	title VARCHAR(45) NOT NULL,
	subtitle VARCHAR(64) NOT NULL,
	description VARCHAR(64) NOT NULL,
	max_file_size INT NOT NULL,
	max_threads SMALLINT NOT NULL,
	default_style VARCHAR(45) NOT NULL,
	locked BOOL NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	anonymous_name VARCHAR(45) NOT NULL DEFAULT 'Anonymous',
	force_anonymous BOOL NOT NULL,
	autosage_after SMALLINT NOT NULL,
	no_images_after SMALLINT NOT NULL,
	max_message_length SMALLINT NOT NULL,
	min_message_length SMALLINT NOT NULL,
	allow_embeds BOOL NOT NULL,
	redirect_to_thread BOOL NOT NULL,
	require_file BOOL NOT NULL,
	enable_catalog BOOL NOT NULL,
	CONSTRAINT boards_section_id_fk
		FOREIGN KEY(section_id) REFERENCES DBPREFIXsections(id),
	CONSTRAINT boards_dir_unique UNIQUE(dir),
	CONSTRAINT boards_uri_unique UNIQUE(uri)
);

CREATE TABLE DBPREFIXthreads(
	id BIGSERIAL PRIMARY KEY,
	board_id BIGINT NOT NULL,
	locked BOOL NOT NULL DEFAULT FALSE,
	stickied BOOL NOT NULL DEFAULT FALSE,
	anchored BOOL NOT NULL DEFAULT FALSE,
	cyclical BOOL NOT NULL DEFAULT FALSE,
	last_bump TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	is_deleted BOOL NOT NULL DEFAULT FALSE,
	CONSTRAINT threads_board_id_fk
		FOREIGN KEY(board_id) REFERENCES DBPREFIXboards(id) ON DELETE CASCADE
);

CREATE INDEX thread_deleted_index ON DBPREFIXthreads(is_deleted);

CREATE TABLE DBPREFIXposts(
	id BIGSERIAL PRIMARY KEY,
	thread_id BIGINT NOT NULL,
	is_top_post BOOL NOT NULL DEFAULT FALSE,
	ip INET NOT NULL,
	created_on TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	name VARCHAR(50) NOT NULL DEFAULT '',
	tripcode VARCHAR(10) NOT NULL DEFAULT '',
	is_role_signature BOOL NOT NULL DEFAULT FALSE,
	email VARCHAR(50) NOT NULL DEFAULT '',
	subject VARCHAR(100) NOT NULL DEFAULT '',
	message TEXT NOT NULL,
	message_raw TEXT NOT NULL,
	password TEXT NOT NULL,
	deleted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	is_deleted BOOL NOT NULL DEFAULT FALSE,
	banned_message TEXT,
	flag VARCHAR(45) NOT NULL DEFAULT '',
	country VARCHAR(80) NOT NULL DEFAULT '',
	CONSTRAINT posts_thread_id_fk
		FOREIGN KEY(thread_id) REFERENCES DBPREFIXthreads(id) ON DELETE CASCADE
);

CREATE INDEX top_post_index ON DBPREFIXposts(is_top_post);

CREATE TABLE DBPREFIXfiles(
	id BIGSERIAL PRIMARY KEY,
	post_id BIGINT NOT NULL,
	file_order INT NOT NULL,
	original_filename VARCHAR(255) NOT NULL,
	filename VARCHAR(45) NOT NULL,
	checksum TEXT NOT NULL,
	file_size INT NOT NULL,
	is_spoilered BOOL NOT NULL,
	thumbnail_width INT NOT NULL,
	thumbnail_height INT NOT NULL,
	width INT NOT NULL,
	height INT NOT NULL,
	CONSTRAINT files_post_id_fk
		FOREIGN KEY(post_id) REFERENCES DBPREFIXposts(id) ON DELETE CASCADE,
	CONSTRAINT files_post_id_file_order_unique UNIQUE(post_id, file_order)
);

CREATE TABLE DBPREFIXstaff(
	id BIGSERIAL PRIMARY KEY,
	username VARCHAR(45) NOT NULL,
	password_checksum VARCHAR(120) NOT NULL,
	global_rank INT,
	added_on TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_login TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	is_active BOOL NOT NULL DEFAULT TRUE,
	CONSTRAINT staff_username_unique UNIQUE(username)
);

CREATE TABLE DBPREFIXsessions(
	id BIGSERIAL PRIMARY KEY,
	staff_id BIGINT NOT NULL,
	expires TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	data VARCHAR(45) NOT NULL,
	CONSTRAINT sessions_staff_id_fk
		FOREIGN KEY(staff_id) REFERENCES DBPREFIXstaff(id) ON DELETE CASCADE
);

CREATE TABLE DBPREFIXboard_staff(
	board_id BIGINT NOT NULL,
	staff_id BIGINT NOT NULL,
	CONSTRAINT board_staff_board_id_fk
		FOREIGN KEY(board_id) REFERENCES DBPREFIXboards(id) ON DELETE CASCADE,
	CONSTRAINT board_staff_staff_id_fk
		FOREIGN KEY(staff_id) REFERENCES DBPREFIXstaff(id) ON DELETE CASCADE,
	CONSTRAINT board_staff_pk PRIMARY KEY (board_id,staff_id)
);

CREATE TABLE DBPREFIXannouncements(
	id BIGSERIAL PRIMARY KEY,
	staff_id BIGINT NOT NULL,
	subject VARCHAR(45) NOT NULL,
	message TEXT NOT NULL,
	timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT announcements_staff_id_fk FOREIGN KEY(staff_id) REFERENCES DBPREFIXstaff(id)
);

CREATE TABLE DBPREFIXip_ban(
	id BIGSERIAL PRIMARY KEY,
	staff_id BIGINT NOT NULL,
	board_id BIGINT,
	banned_for_post_id BIGINT,
	copy_post_text TEXT NOT NULL,
	is_thread_ban BOOL NOT NULL,
	is_active BOOL NOT NULL,
	range_start INET NOT NULL,
	range_end INET NOT NULL,
	issued_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	appeal_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	expires_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	permanent BOOL NOT NULL,
	staff_note VARCHAR(255) NOT NULL,
	message TEXT NOT NULL,
	can_appeal BOOL NOT NULL,
	CONSTRAINT ip_ban_board_id_fk
		FOREIGN KEY(board_id) REFERENCES DBPREFIXboards(id) ON DELETE CASCADE,
	CONSTRAINT ip_ban_staff_id_fk
		FOREIGN KEY(staff_id) REFERENCES DBPREFIXstaff(id),
	CONSTRAINT ip_ban_banned_for_post_id_fk
		FOREIGN KEY(banned_for_post_id) REFERENCES DBPREFIXposts(id)
		ON DELETE SET NULL
);

CREATE TABLE DBPREFIXip_ban_audit(
	ip_ban_id BIGINT NOT NULL,
	timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	staff_id BIGINT NOT NULL,
	is_active BOOL NOT NULL,
	is_thread_ban BOOL NOT NULL,
	expires_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	appeal_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	permanent BOOL NOT NULL,
	staff_note VARCHAR(255) NOT NULL,
	message TEXT NOT NULL,
	can_appeal BOOL NOT NULL,
	PRIMARY KEY(ip_ban_id, timestamp),
	CONSTRAINT ip_ban_audit_ip_ban_id_fk
		FOREIGN KEY(ip_ban_id) REFERENCES DBPREFIXip_ban(id) ON DELETE CASCADE,
	CONSTRAINT ip_ban_audit_staff_id_fk
		FOREIGN KEY(staff_id) REFERENCES DBPREFIXstaff(id)
);

CREATE TABLE DBPREFIXip_ban_appeals(
	id BIGSERIAL PRIMARY KEY,
	staff_id BIGINT,
	ip_ban_id BIGINT NOT NULL,
	appeal_text TEXT NOT NULL,
	staff_response TEXT,
	is_denied BOOL NOT NULL,
	CONSTRAINT ip_ban_appeals_staff_id_fk
		FOREIGN KEY(staff_id) REFERENCES DBPREFIXstaff(id),
	CONSTRAINT ip_ban_appeals_ip_ban_id_fk
		FOREIGN KEY(ip_ban_id) REFERENCES DBPREFIXip_ban(id) ON DELETE CASCADE
);

CREATE TABLE DBPREFIXip_ban_appeals_audit(
	appeal_id BIGINT NOT NULL,
	timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	staff_id BIGINT,
	appeal_text TEXT NOT NULL,
	staff_response TEXT,
	is_denied BOOL NOT NULL,
	PRIMARY KEY(appeal_id, timestamp),
	CONSTRAINT ip_ban_appeals_audit_staff_id_fk
		FOREIGN KEY(staff_id) REFERENCES DBPREFIXstaff(id),
	CONSTRAINT ip_ban_appeals_audit_appeal_id_fk
		FOREIGN KEY(appeal_id) REFERENCES DBPREFIXip_ban_appeals(id)
		ON DELETE CASCADE
);

CREATE TABLE DBPREFIXreports(
	id BIGSERIAL PRIMARY KEY,
	handled_by_staff_id BIGINT,
	post_id BIGINT NOT NULL,
	ip INET NOT NULL,
	reason TEXT NOT NULL,
	is_cleared BOOL NOT NULL,
	CONSTRAINT reports_handled_by_staff_id_fk
		FOREIGN KEY(handled_by_staff_id) REFERENCES DBPREFIXstaff(id),
	CONSTRAINT reports_post_id_fk
		FOREIGN KEY(post_id) REFERENCES DBPREFIXposts(id) ON DELETE CASCADE
);

CREATE TABLE DBPREFIXreports_audit(
	report_id BIGINT NOT NULL,
	timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	handled_by_staff_id BIGINT,
	is_cleared BOOL NOT NULL,
	CONSTRAINT reports_audit_handled_by_staff_id_fk
		FOREIGN KEY(handled_by_staff_id) REFERENCES DBPREFIXstaff(id),
	CONSTRAINT reports_audit_report_id_fk
		FOREIGN KEY(report_id) REFERENCES DBPREFIXreports(id) ON DELETE CASCADE
);

CREATE TABLE DBPREFIXfile_ban(
	id BIGSERIAL PRIMARY KEY,
	board_id BIGINT,
	staff_id BIGINT NOT NULL,
	staff_note VARCHAR(255) NOT NULL,
	issued_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	checksum TEXT NOT NULL,
	fingerprinter VARCHAR(64),
	ban_ip BOOL NOT NULL,
	ban_ip_message TEXT,
	CONSTRAINT file_ban_board_id_fk
		FOREIGN KEY(board_id) REFERENCES DBPREFIXboards(id) ON DELETE CASCADE,
	CONSTRAINT file_ban_staff_id_fk
		FOREIGN KEY(staff_id) REFERENCES DBPREFIXstaff(id)
);

CREATE TABLE DBPREFIXfilters(
	id BIGSERIAL PRIMARY KEY,
	staff_id BIGINT,
	staff_note VARCHAR(255) NOT NULL,
	issued_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	match_action VARCHAR(45) NOT NULL DEFAULT 'replace',
	match_detail TEXT NOT NULL,
	CONSTRAINT filters_staff_id_fk
		FOREIGN KEY(staff_id) REFERENCES DBPREFIXstaff(id)
		ON DELETE SET NULL
);

CREATE TABLE DBPREFIXfilter_boards(
	id BIGSERIAL PRIMARY KEY,
	filter_id BIGINT NOT NULL,
	board_id BIGINT NOT NULL,
	CONSTRAINT filter_boards_filter_id_fk
		FOREIGN KEY(filter_id) REFERENCES DBPREFIXfilters(id)
		ON DELETE CASCADE,
	CONSTRAINT filter_boards_board_id_fk
		FOREIGN KEY(board_id) REFERENCES DBPREFIXboards(id)
		ON DELETE CASCADE
);

CREATE TABLE DBPREFIXfilter_conditions(
	id BIGSERIAL PRIMARY KEY,
	filter_id BIGINT NOT NULL,
	is_regex SMALLINT NOT NULL,
	search VARCHAR(75) NOT NULL,
	field VARCHAR(75) NOT NULL,
	CONSTRAINT filter_conditions_filter_id_fk
		FOREIGN KEY(filter_id) REFERENCES DBPREFIXfilters(id)
		ON DELETE CASCADE,
	CONSTRAINT wordfilter_conditions_search_check CHECK (search <> '')
);

CREATE TABLE DBPREFIXfilter_hits(
	id BIGSERIAL PRIMARY KEY,
	condition_id BIGINT NOT NULL,
	post_data TEXT,
	CONSTRAINT filter_hits_condition_id_fk
		FOREIGN KEY(condition_id) REFERENCES DBPREFIXfilter_conditions(id)
		ON DELETE CASCADE
);

CREATE TABLE DBPREFIXwordfilters(
	id BIGSERIAL PRIMARY KEY,
	staff_id BIGINT NOT NULL,
	staff_note VARCHAR(255) NOT NULL,
	issued_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	search VARCHAR(75) NOT NULL,
	is_regex BOOL NOT NULL,
	change_to VARCHAR(75) NOT NULL,
	CONSTRAINT wordfilters_staff_id_fk
		FOREIGN KEY(staff_id) REFERENCES DBPREFIXstaff(id),
	CONSTRAINT wordfilters_search_check CHECK (search <> '')
);

CREATE TABLE DBPREFIXwordfilter_boards(
	id BIGSERIAL PRIMARY KEY,
	filter_id BIGINT NOT NULL,
	board_id BIGINT NOT NULL,
	CONSTRAINT wordfilter_boards_filter_id_fk
		FOREIGN KEY(filter_id) REFERENCES DBPREFIXfilters(id)
		ON DELETE CASCADE,
	CONSTRAINT wordfilter_boards_board_id_fk
		FOREIGN KEY(board_id) REFERENCES DBPREFIXboards(id)
		ON DELETE CASCADE
);

INSERT INTO DBPREFIXdatabase_version(component, version)
	VALUES('gochan', 3);
