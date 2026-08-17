-- 007_fix_nik_length.sql -- NIP is 18 digits
ALTER TABLE users ALTER COLUMN nik TYPE varchar(18);
