-- Valores nutricionais de alimentos são sempre por 100 g.
CREATE TABLE alimentos (
    id         INTEGER PRIMARY KEY,
    nome       TEXT    NOT NULL,
    nome_busca TEXT    NOT NULL,             -- minúsculo e sem acento, para a busca
    categoria  TEXT    NOT NULL DEFAULT '',
    fonte      TEXT    NOT NULL CHECK (fonte IN ('taco', 'usuario', 'receita')),
    taco_id    INTEGER UNIQUE,
    kcal       REAL    NOT NULL CHECK (kcal >= 0),
    prot       REAL    NOT NULL CHECK (prot >= 0),
    carb       REAL    NOT NULL CHECK (carb >= 0),
    gord       REAL    NOT NULL CHECK (gord >= 0),
    fibra      REAL    NOT NULL DEFAULT 0 CHECK (fibra >= 0)
);

CREATE TABLE porcoes (
    id          INTEGER PRIMARY KEY,
    alimento_id INTEGER NOT NULL REFERENCES alimentos (id) ON DELETE CASCADE,
    nome        TEXT    NOT NULL,
    gramas      REAL    NOT NULL CHECK (gramas > 0),
    UNIQUE (alimento_id, nome)
);

-- Os macros ficam congelados no registro: editar o alimento não muda o histórico.
CREATE TABLE lancamentos (
    id          INTEGER PRIMARY KEY,
    data        TEXT    NOT NULL,             -- AAAA-MM-DD
    refeicao    TEXT    NOT NULL CHECK (refeicao IN ('cafe', 'almoco', 'lanche', 'jantar')),
    alimento_id INTEGER NOT NULL REFERENCES alimentos (id),
    gramas      REAL    NOT NULL CHECK (gramas > 0),
    kcal        REAL    NOT NULL,
    prot        REAL    NOT NULL,
    carb        REAL    NOT NULL,
    gord        REAL    NOT NULL,
    origem      TEXT    NOT NULL CHECK (origem IN ('manual', 'agente')),
    estimado    INTEGER NOT NULL DEFAULT 0,
    criado_em   TEXT    NOT NULL,
    removido_em TEXT                          -- remoção lógica, para o desfazer
);
CREATE INDEX lancamentos_data ON lancamentos (data) WHERE removido_em IS NULL;

CREATE TABLE metas (
    vigente_desde TEXT PRIMARY KEY,           -- AAAA-MM-DD
    kcal          REAL NOT NULL,
    prot          REAL NOT NULL,
    carb          REAL NOT NULL,
    gord          REAL NOT NULL
);

-- antes/depois guardam o lançamento em JSON.
CREATE TABLE auditoria (
    id            INTEGER PRIMARY KEY,
    lancamento_id INTEGER NOT NULL REFERENCES lancamentos (id),
    acao          TEXT    NOT NULL CHECK (acao IN ('criar', 'editar', 'remover', 'desfazer')),
    antes         TEXT,
    depois        TEXT,
    origem        TEXT    NOT NULL CHECK (origem IN ('manual', 'agente')),
    em            TEXT    NOT NULL,
    desfeito      INTEGER NOT NULL DEFAULT 0
);
