-- A fibra passa a ser congelada no lançamento. Os antigos são preenchidos a partir do
-- alimento: exato para a TACO, aproximado para receitas editadas depois do registro.
ALTER TABLE lancamentos ADD COLUMN fibra REAL NOT NULL DEFAULT 0;
UPDATE lancamentos SET fibra = round(
    gramas * (SELECT a.fibra FROM alimentos a WHERE a.id = lancamentos.alimento_id) / 100, 1);

-- Uma pesagem por dia; registrar de novo no mesmo dia substitui.
CREATE TABLE peso (
    data TEXT PRIMARY KEY,                 -- AAAA-MM-DD
    kg   REAL NOT NULL CHECK (kg > 0)
);
