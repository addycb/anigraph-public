-- Store OP video embeddings (multiple per anime)
CREATE TABLE IF NOT EXISTS anime_op_embedding (
    anime_id INTEGER NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
    op_number INTEGER NOT NULL DEFAULT 1,
    title_op VARCHAR(200) NOT NULL,
    embedding float4[] NOT NULL,
    PRIMARY KEY (anime_id, op_number)
);

-- Precomputed similar OPs (top 20 per source OP)
CREATE TABLE IF NOT EXISTS anime_similar_op (
    anime_id INTEGER NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
    op_number INTEGER NOT NULL DEFAULT 1,
    similar_anime_id INTEGER NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
    similar_op_number INTEGER NOT NULL DEFAULT 1,
    similarity NUMERIC(10,6) NOT NULL,
    rank INTEGER NOT NULL,
    PRIMARY KEY (anime_id, op_number, similar_anime_id, similar_op_number)
);
CREATE INDEX IF NOT EXISTS idx_similar_op_anime ON anime_similar_op(anime_id, op_number, rank);
