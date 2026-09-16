-- Capture only new events, in the transaction that commits the business result.
-- Never backfill historical messages or copy target credentials into the outbox.
create table notification_events (
 id uuid primary key default gen_random_uuid(),
 source_key text not null unique,
 event jsonb not null,
 created_at timestamptz not null default now()
);
create table notification_deliveries (
 id uuid primary key default gen_random_uuid(),
 event_id uuid not null references notification_events(id),
 target_id uuid not null,
 target_name text not null,
 target_type text not null,
 target_revision timestamptz not null,
 state text not null default 'pending' check(state in ('pending','sending','retry','accepted','failed','uncertain','cancelled')),
 attempts integer not null default 0,
 next_attempt_at timestamptz not null default now(),
 run_token uuid,
 status_code integer,
 message text not null default '',
 created_at timestamptz not null default now(),
 updated_at timestamptz not null default now(),
 unique(event_id,target_id)
);
create index notification_deliveries_due_idx on notification_deliveries(next_attempt_at,id) where state in ('pending','retry','sending');
create index notification_deliveries_history_idx on notification_deliveries(created_at desc,id desc);
create table notification_health_states (
 check_id text primary key,
 severity text not null,
 updated_at timestamptz not null default now()
);
create function enqueue_native_notification(source text, payload jsonb) returns void language plpgsql as $$
declare event_id uuid;
begin
 insert into notification_events(source_key,event) values(source,payload)
 on conflict(source_key) do nothing returning id into event_id;
 if event_id is null then return; end if;
 insert into notification_deliveries(event_id,target_id,target_name,target_type,target_revision)
 select event_id,id,name,type,updated_at from notification_targets
 where enabled and case payload->>'type'
 when 'grab' then on_grab when 'import' then on_import when 'upgrade' then on_upgrade
 when 'downloadFailure' then on_download_failure when 'healthIssue' then on_health_issue else false end;
end $$;
create function capture_native_history_notification() returns trigger language plpgsql as $$
declare kind text; title text; subject text; fields jsonb;
begin
 if new.event_type not in ('release_grabbed','book_imported') then return new; end if;
 if new.event_type='release_grabbed' then
   kind := case when new.data->>'trigger'='upgrade' then 'upgrade' else 'grab' end;
   title := case when kind='upgrade' then 'Book upgrade grabbed' else 'Book grabbed' end;
   subject := coalesce(new.data->>'title','');
   fields := jsonb_build_object('source',coalesce(new.data->>'trigger','acquisition'),'client',coalesce(new.data->>'client',''),'downloadId',coalesce(new.data->>'downloadId',''));
 else
   kind := 'import'; title := 'Book imported';
   subject := coalesce((select wi.title from wanted_items wi where wi.id::text=new.entity_id),new.data->>'title','');
   fields := jsonb_build_object('source',coalesce(new.data->>'sourceKind','import'),'format',coalesce(new.data->>'format',''),'operationId',coalesce(new.data->>'operationId',''));
 end if;
 if new.entity_type='wanted_item' then fields := fields || jsonb_build_object('wantedId',new.entity_id); end if;
 if subject<>'' then title := title || ': ' || subject; end if;
 perform enqueue_native_notification('history:'||new.id::text,jsonb_build_object('type',kind,'title',title,'message',case when kind='import' then coalesce(new.data->'paths'->>0,new.message) else new.message end,'fields',fields));
 return new;
end $$;
create trigger notification_history_capture after insert on history_events for each row execute function capture_native_history_notification();
create function capture_native_download_failure() returns trigger language plpgsql as $$
begin
 if new.failed_at is not null and (tg_op='INSERT' or old.failed_at is null) then
   perform enqueue_native_notification('download-failure:'||new.id::text||':'||gen_random_uuid()::text,
     jsonb_build_object('type','downloadFailure','title','Download failed: '||new.name,'message',new.failure_reason,
       'fields',jsonb_build_object('source','download-recovery','client',new.client,'downloadId',coalesce(new.external_id,''))));
 end if;
 return new;
end $$;
create trigger notification_failure_capture after insert or update of failed_at on downloads for each row execute function capture_native_download_failure();
create table notification_delivery_attempts (
 id uuid primary key,
 delivery_id uuid not null references notification_deliveries(id),
 state text not null,
 status_code integer,
 message text not null default '',
 started_at timestamptz not null default now(),
 finished_at timestamptz
);
create index notification_attempts_delivery_idx on notification_delivery_attempts(delivery_id,started_at desc);
create table notification_delivery_actions (
 id uuid primary key default gen_random_uuid(),
 delivery_id uuid not null references notification_deliveries(id),
 action text not null,
 previous_state text not null,
 target_revision timestamptz not null,
 created_at timestamptz not null default now()
);
