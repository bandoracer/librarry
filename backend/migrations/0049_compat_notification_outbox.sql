-- Extend new-event capture to persisted Readarr webhook resources. Existing
-- events/deliveries are not replayed or fanned out to compatibility targets.
alter table notification_events add column compat_context jsonb not null default '{}';
alter table notification_deliveries add column target_kind text not null default 'native' check(target_kind in ('native','compat'));
alter table notification_deliveries drop constraint notification_deliveries_event_id_target_id_key;
alter table notification_deliveries add unique(event_id,target_kind,target_id);

create function notification_payload_bool(payload jsonb, field text, fallback boolean) returns boolean language sql immutable as $$
 select case jsonb_typeof(payload->field)
 when 'boolean' then (payload->>field)::boolean
 when 'number' then (payload->>field)::numeric<>0
 when 'string' then case trim(payload->>field) when 'true' then true when 'TRUE' then true when 'True' then true when 't' then true when 'T' then true when '1' then true when 'false' then false when 'FALSE' then false when 'False' then false when 'f' then false when 'F' then false when '0' then false else fallback end
 else fallback end
$$;
create function notification_compat_matches(payload jsonb, event_type text) returns boolean language sql immutable as $$
 select notification_payload_bool(payload,'enable',false)
 and lower(coalesce(nullif(trim(payload->>'implementation'),''),nullif(trim(payload->>'implementationName'),''),'Webhook')) like '%webhook%'
 and case event_type
 when 'grab' then notification_payload_bool(payload,'onGrab',true)
 when 'import' then notification_payload_bool(payload,'onReleaseImport',notification_payload_bool(payload,'onDownload',true))
 when 'upgrade' then notification_payload_bool(payload,'onUpgrade',true)
 when 'downloadFailure' then notification_payload_bool(payload,'onDownloadFailure',true)
 when 'healthIssue' then notification_payload_bool(payload,'onHealthIssue',false)
 else false end
$$;
-- Explicit field maps prevent raw settings, release URLs or provider blobs from
-- entering a delayed outbound request. Adding a DB column cannot expand it.
create function notification_snapshot(record jsonb, fields jsonb) returns jsonb language sql immutable as $$
 select coalesce(jsonb_object_agg(f.key,record->f.value) filter(where record->f.value is not null and record->f.value<>'null'::jsonb),'{}') from jsonb_each_text(fields) f
$$;
create function notification_uuid(value text) returns uuid language plpgsql immutable as $$
begin return value::uuid; exception when invalid_text_representation then return null; end $$;
create function capture_compat_notification_context() returns trigger language plpgsql as $$
declare history history_events%rowtype; download downloads%rowtype; wanted wanted_items%rowtype; release releases%rowtype; files jsonb; wanted_id text; data jsonb := '{}';
begin
 if new.source_key like 'history:%' then
   select * into history from history_events where id=notification_uuid(substring(new.source_key from 9));
   data:=coalesce(history.data,'{}');
   if history.entity_type='wanted_item' then wanted_id:=history.entity_id; end if;
 end if;
 if data->>'downloadRecordId' is not null and data->>'downloadRecordId'<>'' then
   select * into download from downloads where id=notification_uuid(data->>'downloadRecordId');
 elsif new.event->'fields'->>'downloadId' is not null then
   select * into download from downloads where client=new.event->'fields'->>'client' and external_id=new.event->'fields'->>'downloadId';
 end if;
 select * into release from releases where id=case when history.event_type='book_imported' then notification_uuid(data->>'releaseId') else coalesce(notification_uuid(data->>'releaseId'),download.release_id) end;
 wanted_id:=coalesce(wanted_id,(select a.wanted_item_id::text from acquisition_intents a where a.id=download.acquisition_intent_id),release.wanted_item_id::text);
 select * into wanted from wanted_items where id=notification_uuid(wanted_id);
 if wanted.id is not null then
   new.compat_context:=new.compat_context||jsonb_build_object('wantedItem',notification_snapshot(to_jsonb(wanted),'{"id":"id","workId":"work_id","editionId":"edition_id","title":"title","authorName":"author_name","format":"wanted_format","qualityProfile":"quality_profile","status":"status","monitored":"monitored","rootFolderId":"root_folder_id","series":"series","seriesPosition":"series_position","firstPublishYear":"first_publish_year","releaseDate":"release_date","sourceProvider":"metadata_provider","sourceKey":"source_key","currentReleaseId":"current_release_id","currentReleaseScore":"current_release_score","createdAt":"created_at","updatedAt":"updated_at"}'));
 end if;
 if download.id is not null then
   new.compat_context:=new.compat_context||jsonb_build_object('download',notification_snapshot(to_jsonb(download),'{"id":"external_id","client":"client","name":"name","state":"state","progress":"progress","savePath":"save_path","category":"category","sizeBytes":"size_bytes","downloadedBytes":"downloaded_bytes","uploadedBytes":"uploaded_bytes","downloadRate":"download_rate","uploadRate":"upload_rate","etaSeconds":"eta_seconds","ratio":"ratio","seeders":"seeders","peers":"peers","addedAt":"added_at","completedAt":"completed_at","lastActivityAt":"last_activity_at","lastSeenAt":"last_seen_at","importStatus":"import_status","importedFileId":"imported_file_id","importedAt":"imported_at","failureReason":"failure_reason","failedAt":"failed_at","retryCount":"retry_count","replacementId":"replacement_external_id","releaseId":"release_id","acquisitionId":"acquisition_intent_id"}'));
 end if;
 if release.id is not null and (wanted.id is null or release.wanted_item_id=wanted.id) then
   new.compat_context:=new.compat_context||jsonb_build_object('release',notification_snapshot(to_jsonb(release),'{"id":"id","wantedItemId":"wanted_item_id","sourceId":"source_id","infoHash":"info_hash","indexer":"indexer","title":"title","protocol":"protocol","sizeBytes":"size_bytes","seeders":"seeders","leechers":"leechers","score":"score","approved":"approved","rejectedReason":"rejected_reason","publishedAt":"published_at","searchedAt":"searched_at","createdAt":"created_at"}')||case when jsonb_typeof(data->'score')='number' then jsonb_build_object('score',data->'score') else '{}'::jsonb end);
 end if;
 if jsonb_typeof(data->'fileIds')='array' then
   select coalesce(jsonb_agg(notification_snapshot(to_jsonb(f),'{"id":"id","editionId":"edition_id","mediaFormat":"media_format","path":"path","sourcePath":"source_path","title":"title","authorName":"author_name","extension":"extension","sizeBytes":"size_bytes","checksum":"checksum","importStatus":"import_status","presenceState":"presence_state","modifiedAt":"modified_at","createdAt":"created_at","updatedAt":"updated_at"}') order by i.ordinality),'[]') into files
   from jsonb_array_elements_text(data->'fileIds') with ordinality i(id,ordinality) join files f on f.id=notification_uuid(i.id);
   new.compat_context:=new.compat_context||jsonb_build_object('files',files,'operationId',coalesce(data->>'operationId',''),'conflictAction',coalesce(data->>'conflictAction',''));
 end if;
 new.compat_context:=new.compat_context||jsonb_build_object('extra',notification_snapshot(data,'{"currentScore":"currentScore","cutoffScore":"cutoffScore"}'));
 return new;
end $$;
create trigger notification_compat_context before insert on notification_events for each row execute function capture_compat_notification_context();
create function capture_compat_notification_targets() returns trigger language plpgsql as $$
begin
 insert into notification_deliveries(event_id,target_kind,target_id,target_name,target_type,target_revision)
 select new.id,'compat',r.id,coalesce(nullif(r.name,''),nullif(r.payload->>'name',''),'Readarr webhook'),'readarrWebhook',r.updated_at from compat_resources r
 where r.resource_type='notification' and r.deleted_at is null and notification_compat_matches(r.payload,new.event->>'type');
 return new;
end $$;
create trigger notification_compat_targets after insert on notification_events for each row execute function capture_compat_notification_targets();
