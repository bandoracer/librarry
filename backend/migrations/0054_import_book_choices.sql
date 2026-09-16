-- Local identity pickers stay independent of live download/file projections.
create index wanted_book_choices_page_idx on wanted_items(created_at desc,id desc) where status not in ('removed','ignored');
