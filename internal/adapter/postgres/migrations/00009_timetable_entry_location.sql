-- +goose Up
-- Own location of a timetable entry (Story 5.1). An entry may refer to a
-- stored location other than the one of its event, such as a concert in the
-- church at a Christmas market. NULL means the entry takes place at the
-- location of its event, so existing entries keep their meaning and no data
-- changes. The core validates the reference; the database only keeps it
-- from pointing at nothing, and a location an entry refers to cannot be
-- deleted (AD-6).
-- Expand step (AD-17): the column is nullable without default, so code
-- without location_id keeps working after a rollback.
-- Once applied, this file must never change (AD-17).
ALTER TABLE timetable_entries
    ADD COLUMN location_id uuid
        CONSTRAINT timetable_entries_location_id_fkey REFERENCES locations (id) ON DELETE RESTRICT;

-- Counting the entries of a location, and the foreign key's check on
-- deleting a location, look entries up by location.
CREATE INDEX timetable_entries_location_id_idx ON timetable_entries (location_id);
