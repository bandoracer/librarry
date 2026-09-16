-- Persist release selection before client IO, and repair its local projections
-- independently of remote acceptance. Legacy rows have no proven selection.
alter table acquisition_intents add column selection jsonb not null default '{}'::jsonb;
alter table acquisition_intents add column bookkeeping_required boolean not null default false;
alter table acquisition_intents alter column bookkeeping_required set default true;
alter table acquisition_intents add column bookkeeping_at timestamptz;
create index acquisition_intents_bookkeeping on acquisition_intents(created_at,id)
  where state='accepted' and bookkeeping_required and bookkeeping_at is null;
alter table downloads add column acquisition_intent_id uuid references acquisition_intents(id) on delete set null;
create index downloads_acquisition_intent on downloads(acquisition_intent_id) where acquisition_intent_id is not null;
