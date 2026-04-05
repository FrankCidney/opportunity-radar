ALTER TABLE app_settings
ADD COLUMN desired_roles TEXT[] NOT NULL DEFAULT '{}',
ADD COLUMN experience_level TEXT NOT NULL DEFAULT '',
ADD COLUMN current_skills TEXT[] NOT NULL DEFAULT '{}',
ADD COLUMN growth_skills TEXT[] NOT NULL DEFAULT '{}',
ADD COLUMN locations TEXT[] NOT NULL DEFAULT '{}',
ADD COLUMN work_modes TEXT[] NOT NULL DEFAULT '{}',
ADD COLUMN avoid_terms TEXT[] NOT NULL DEFAULT '{}';
