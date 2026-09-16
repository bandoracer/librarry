-- The same release also has a single active request when a raw manual grab and
-- a book-associated worker use different scope keys.
create unique index acquisition_intents_active_request on acquisition_intents(request_key) where state <> 'released';
create index acquisition_intents_download_identity on acquisition_intents(client,external_id) where external_id <> '';
