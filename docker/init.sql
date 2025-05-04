CREATE TABLE worlds (
  id          SERIAL PRIMARY KEY,
  name        TEXT    NOT NULL,
  description TEXT    NOT NULL,
  population  INTEGER NOT NULL,
  climate     TEXT    NOT NULL,
  features    TEXT[]  NOT NULL,
  theme       TEXT    NOT NULL DEFAULT 'fantasy',
  created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
  fauna       TEXT[],
  flora       TEXT[],
  cultures    TEXT[],
  dangers     TEXT[],
  languages   TEXT[]
);

CREATE INDEX idx_worlds_theme ON worlds(theme);
CREATE INDEX idx_worlds_climate ON worlds(climate);
CREATE INDEX idx_worlds_created_at ON worlds(created_at);
CREATE INDEX idx_worlds_name_desc ON worlds
       USING gin(to_tsvector('english', name || ' ' || description));

CREATE VIEW popular_world_types AS
SELECT theme, climate, COUNT(*) as count
FROM worlds
GROUP BY theme, climate
ORDER BY count DESC;
