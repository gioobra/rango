-- Uma receita é um alimento (fonte = 'receita') cujos valores por 100 g saem dos ingredientes.
CREATE TABLE receitas (
    alimento_id  INTEGER PRIMARY KEY REFERENCES alimentos (id) ON DELETE CASCADE,
    rendimento_g REAL    NOT NULL CHECK (rendimento_g > 0),  -- peso pronto da panela inteira
    n_porcoes    INTEGER NOT NULL CHECK (n_porcoes > 0),
    nome_porcao  TEXT    NOT NULL,                          -- "1 marmita", vira uma porção do alimento
    criado_em    TEXT    NOT NULL,
    editado_em   TEXT    NOT NULL
);

CREATE TABLE receita_itens (
    id          INTEGER PRIMARY KEY,
    receita_id  INTEGER NOT NULL REFERENCES receitas (alimento_id) ON DELETE CASCADE,
    alimento_id INTEGER NOT NULL REFERENCES alimentos (id),
    gramas      REAL    NOT NULL CHECK (gramas > 0)
);
CREATE INDEX receita_itens_receita ON receita_itens (receita_id);
