ALTER TABLE sample_data_generations
  ADD COLUMN IF NOT EXISTS accounting_book_id UUID;

UPDATE sample_data_generations generation
SET accounting_book_id=project.accounting_book_id
FROM projects project
WHERE project.id=generation.project_id
  AND generation.accounting_book_id IS NULL;

ALTER TABLE sample_data_generations
  DROP CONSTRAINT IF EXISTS sample_data_generations_accounting_book_fk;

ALTER TABLE sample_data_generations
  ADD CONSTRAINT sample_data_generations_accounting_book_fk
  FOREIGN KEY (accounting_book_id) REFERENCES accounting_books(id) ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_sample_data_generations_sources
  ON sample_data_generations(template_id,accounting_book_id,created_at DESC);
