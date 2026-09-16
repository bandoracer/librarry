-- Journal a lease-specific temporary path before creating it. On recovery, only
-- this exact recorded path may be reclaimed; never sweep a directory by prefix.
alter table import_operation_files add column stage_path text not null default '';
alter table import_operation_files add column stage_lease_token uuid;
alter table import_operation_files add constraint import_stage_identity check ((stage_path='') = (stage_lease_token is null));
