-- Author creation defaults apply only to newly tracked books. Review candidates
-- retain the destination selected when they were last evaluated.
alter table author_subscriptions
  add column root_folder_id uuid references root_folders(id) on delete set null;

alter table author_metadata_reviews
  add column root_folder_id uuid references root_folders(id) on delete set null;

create index author_subscriptions_root_folder_idx on author_subscriptions(root_folder_id)
  where root_folder_id is not null;
create index author_metadata_reviews_root_folder_idx on author_metadata_reviews(root_folder_id)
  where root_folder_id is not null;
