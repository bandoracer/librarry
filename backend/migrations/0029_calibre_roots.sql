-- Calibre-managed native root folders: per-root Calibre Content Server
-- connection settings. When use_calibre is set, imports into the root hand the
-- file to the Calibre add-book endpoint instead of the move/hardlink+naming
-- path, and rename operations skip files under the root.

alter table root_folders add column if not exists use_calibre boolean not null default false;
alter table root_folders add column if not exists calibre_host text not null default '';
alter table root_folders add column if not exists calibre_port integer not null default 0;
alter table root_folders add column if not exists calibre_url_base text not null default '';
alter table root_folders add column if not exists calibre_username text not null default '';
alter table root_folders add column if not exists calibre_password text not null default '';
alter table root_folders add column if not exists calibre_library text not null default '';
alter table root_folders add column if not exists calibre_convert_formats text not null default '';
alter table root_folders add column if not exists calibre_output_profile text not null default '';
alter table root_folders add column if not exists calibre_use_ssl boolean not null default false;
