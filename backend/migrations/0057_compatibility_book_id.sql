-- Match the numeric book IDs already emitted by the compatibility API. This
-- enables honest numeric-ID ordering; collision handling remains explicit.
create function librarry_book_compat_id(book_id uuid) returns integer
language plpgsql immutable strict parallel safe as $$
declare bytes bytea := convert_to(book_id::text, 'UTF8'); value bigint := 2166136261; position integer;
begin
  for position in 0..length(bytes)-1 loop
    value := ((value # get_byte(bytes, position)::bigint) * 16777619) % 4294967296;
  end loop;
  return (value & 2147483647)::integer;
end
$$;
