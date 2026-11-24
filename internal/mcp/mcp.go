// Package mcp expõe o Rango como servidor MCP (stdio) para o Nutri e outros clientes.
// As ferramentas são finas: validam a entrada, chamam o core e devolvem JSON.
package mcp

import (
	"context"
	"errors"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gioobra/rango/internal/core"
	"github.com/gioobra/rango/internal/store"
)

const instrucoes = `Diário alimentar do Rango. Valores nutricionais vêm da tabela TACO ou de alimentos do usuário.
Sempre use buscar_alimento antes de registrar e prefira as porções salvas do usuário.
Datas em AAAA-MM-DD; sem data, vale hoje. Refeições: cafe, almoco, lanche, jantar.`

// Novo monta o servidor com as ferramentas. Em somenteLeitura, só as de consulta.
func Novo(svc *core.Servico, versao string, somenteLeitura bool) *sdk.Server {
	s := sdk.NewServer(&sdk.Implementation{Name: "rango", Version: versao},
		&sdk.ServerOptions{Instructions: instrucoes, Capabilities: &sdk.ServerCapabilities{}})
	f := &ferramentas{svc: svc}

	leitura := &sdk.ToolAnnotations{ReadOnlyHint: true}
	sdk.AddTool(s, &sdk.Tool{Name: "buscar_alimento", Annotations: leitura,
		Description: "Procura alimentos pelo nome (ignora acentos; todos os termos precisam aparecer, então se não achar tente menos termos). " +
			"Devolve id, valores por 100 g e as porções salvas. Os do usuário vêm primeiro, os crus por último."},
		f.buscarAlimento)
	sdk.AddTool(s, &sdk.Tool{Name: "resumo_dia", Annotations: leitura,
		Description: "Diário de um dia: lançamentos (com id) por refeição, totais, meta e quanto falta."},
		f.resumoDia)
	sdk.AddTool(s, &sdk.Tool{Name: "listar_receitas", Annotations: leitura,
		Description: "Receitas e marmitas salvas, com ingredientes, rendimento, porção e macros da porção. " +
			"Para registrar, use o id da receita como alimento_id em registrar_refeicao."},
		f.listarReceitas)
	sdk.AddTool(s, &sdk.Tool{Name: "metas", Annotations: leitura,
		Description: "Meta diária vigente na data (kcal e gramas de proteína, carboidrato e gordura)."},
		f.metas)
	if somenteLeitura {
		return s
	}

	aditiva := &sdk.ToolAnnotations{DestructiveHint: new(false)}
	destrutiva := &sdk.ToolAnnotations{DestructiveHint: new(true)}
	sdk.AddTool(s, &sdk.Tool{Name: "registrar_refeicao", Annotations: aditiva,
		Description: "Registra um ou mais alimentos numa refeição, de uma vez. Os macros são calculados " +
			"e congelados. Marque estimado quando a quantidade for um palpite. Devolve os lançamentos e o resumo do dia."},
		f.registrar)
	sdk.AddTool(s, &sdk.Tool{Name: "editar_lancamento", Annotations: destrutiva,
		Description: "Muda gramas, refeição ou data de um lançamento. Mudar os gramas reescala os macros."},
		f.editar)
	sdk.AddTool(s, &sdk.Tool{Name: "remover_lancamento", Annotations: destrutiva,
		Description: "Remove um lançamento do diário. Só use depois de o usuário confirmar."},
		f.remover)
	sdk.AddTool(s, &sdk.Tool{Name: "desfazer", Annotations: destrutiva,
		Description: "Desfaz a mudança mais recente no diário (de qualquer origem: web ou Nutri)."},
		f.desfazer)
	sdk.AddTool(s, &sdk.Tool{Name: "definir_porcao", Annotations: aditiva,
		Description: "Salva uma medida caseira do usuário para um alimento (\"1 concha\" = 90 g). " +
			"Se o nome já existe, atualiza os gramas."},
		f.definirPorcao)
	sdk.AddTool(s, &sdk.Tool{Name: "definir_metas", Annotations: destrutiva,
		Description: "Define a meta diária, valendo a partir de hoje."},
		f.definirMetas)
	sdk.AddTool(s, &sdk.Tool{Name: "criar_receita", Annotations: aditiva,
		Description: "Salva uma receita ou marmita: ingredientes (ids de buscar_alimento, em gramas crus como foram " +
			"para a panela), peso pronto se a pessoa pesou e em quantas porções rendeu. Vira um alimento buscável, " +
			"com a porção salva (\"1 marmita\" = peso ÷ porções)."},
		f.criarReceita)
	sdk.AddTool(s, &sdk.Tool{Name: "editar_receita", Annotations: destrutiva,
		Description: "Substitui ingredientes, peso pronto e porções de uma receita. Mande a receita inteira, " +
			"não só o que mudou. Lançamentos já feitos não mudam."},
		f.editarReceita)
	sdk.AddTool(s, &sdk.Tool{Name: "criar_alimento", Annotations: aditiva,
		Description: "Cadastra um alimento que não existe na TACO, com valores por 100 g (de um rótulo, por exemplo). " +
			"Se o rótulo dá os valores por porção, converta para 100 g antes."},
		f.criarAlimento)
	return s
}

// Rodar atende um cliente pelo stdin/stdout até ele desconectar.
func Rodar(ctx context.Context, s *sdk.Server) error {
	return s.Run(ctx, &sdk.StdioTransport{})
}

type ferramentas struct{ svc *core.Servico }

// --- formatos de saída ---

// Nutrientes por 100 g ou de uma quantidade.
type Nutrientes struct {
	Kcal  float64 `json:"kcal"`
	Prot  float64 `json:"prot_g"`
	Carb  float64 `json:"carb_g"`
	Gord  float64 `json:"gord_g"`
	Fibra float64 `json:"fibra_g,omitempty"`
}

// Alimento como o agente vê.
type Alimento struct {
	ID        int64          `json:"id"`
	Nome      string         `json:"nome"`
	Categoria string         `json:"categoria,omitempty"`
	Fonte     string         `json:"fonte"`
	Por100g   Nutrientes     `json:"por_100g"`
	Porcoes   []store.Porcao `json:"porcoes,omitempty"`
}

// Refeicao do dia com seus lançamentos.
type Refeicao struct {
	Refeicao    string             `json:"refeicao"`
	Lancamentos []store.Lancamento `json:"lancamentos"`
	Totais      Nutrientes         `json:"totais"`
}

// Dia resumido.
type Dia struct {
	Data      string      `json:"data"`
	Refeicoes []Refeicao  `json:"refeicoes"`
	Totais    Nutrientes  `json:"totais"`
	Meta      *Nutrientes `json:"meta,omitempty"`
	Restante  *Nutrientes `json:"restante,omitempty"`
}

func deTotais(t core.Totais) Nutrientes {
	return Nutrientes{Kcal: t.Kcal, Prot: t.Prot, Carb: t.Carb, Gord: t.Gord}
}

func deMeta(m store.Meta) *Nutrientes {
	return &Nutrientes{Kcal: m.Kcal, Prot: m.Prot, Carb: m.Carb, Gord: m.Gord}
}

func deDia(d core.Dia) Dia {
	out := Dia{Data: d.Data.Format(core.FormatoData), Totais: deTotais(d.Totais), Refeicoes: []Refeicao{}}
	for _, r := range d.Refeicoes {
		if len(r.Itens) == 0 {
			continue
		}
		out.Refeicoes = append(out.Refeicoes, Refeicao{Refeicao: r.Chave, Lancamentos: r.Itens, Totais: deTotais(r.Totais)})
	}
	if d.TemMeta {
		out.Meta = deMeta(d.Meta)
		r := deTotais(d.Restante())
		out.Restante = &r
	}
	return out
}

func (f *ferramentas) dia(ctx context.Context, data time.Time) (Dia, error) {
	d, err := f.svc.Diario(ctx, data)
	return deDia(d), err
}

// --- ferramentas de leitura ---

type entradaBusca struct {
	Texto  string `json:"texto" jsonschema:"nome ou parte do nome, por exemplo: arroz branco"`
	Limite int    `json:"limite,omitempty" jsonschema:"máximo de resultados (padrão 10, até 50)"`
}

type saidaBusca struct {
	Alimentos []Alimento `json:"alimentos"`
}

func (f *ferramentas) buscarAlimento(ctx context.Context, _ *sdk.CallToolRequest, in entradaBusca) (*sdk.CallToolResult, saidaBusca, error) {
	if in.Limite <= 0 {
		in.Limite = 10
	}
	as, err := f.svc.BuscarAlimentos(ctx, in.Texto, in.Limite)
	if err != nil {
		return nil, saidaBusca{}, err
	}
	out := saidaBusca{Alimentos: []Alimento{}}
	for _, a := range as {
		_, ps, err := f.svc.Alimento(ctx, a.ID)
		if err != nil {
			return nil, saidaBusca{}, err
		}
		out.Alimentos = append(out.Alimentos, Alimento{
			ID: a.ID, Nome: a.Nome, Categoria: a.Categoria, Fonte: a.Fonte, Porcoes: ps,
			Por100g: Nutrientes{Kcal: a.Kcal, Prot: a.Prot, Carb: a.Carb, Gord: a.Gord, Fibra: a.Fibra},
		})
	}
	return nil, out, nil
}

type entradaData struct {
	Data string `json:"data,omitempty" jsonschema:"AAAA-MM-DD; vazio é hoje"`
}

func (f *ferramentas) resumoDia(ctx context.Context, _ *sdk.CallToolRequest, in entradaData) (*sdk.CallToolResult, Dia, error) {
	data, err := f.svc.LerData(in.Data)
	if err != nil {
		return nil, Dia{}, err
	}
	d, err := f.dia(ctx, data)
	return nil, d, err
}

type saidaMetas struct {
	Definida     bool        `json:"definida"`
	VigenteDesde string      `json:"vigente_desde,omitempty"`
	Meta         *Nutrientes `json:"meta,omitempty"`
}

func (f *ferramentas) metas(ctx context.Context, _ *sdk.CallToolRequest, in entradaData) (*sdk.CallToolResult, saidaMetas, error) {
	data, err := f.svc.LerData(in.Data)
	if err != nil {
		return nil, saidaMetas{}, err
	}
	m, ok, err := f.svc.Metas(ctx, data)
	if err != nil || !ok {
		return nil, saidaMetas{}, err
	}
	return nil, saidaMetas{Definida: true, VigenteDesde: m.VigenteDesde, Meta: deMeta(m)}, nil
}

// --- ferramentas de escrita ---

type item struct {
	AlimentoID int64   `json:"alimento_id" jsonschema:"id vindo de buscar_alimento"`
	Gramas     float64 `json:"gramas" jsonschema:"quantidade em gramas"`
	Estimado   bool    `json:"estimado,omitempty" jsonschema:"true se a quantidade foi estimada"`
}

type entradaRegistro struct {
	Data     string `json:"data,omitempty" jsonschema:"AAAA-MM-DD; vazio é hoje"`
	Refeicao string `json:"refeicao" jsonschema:"cafe, almoco, lanche ou jantar"`
	Itens    []item `json:"itens"`
}

// SaidaRegistro é o resultado de registrar_refeicao. A web lê este formato para montar o cartão.
type SaidaRegistro struct {
	Lancamentos []store.Lancamento `json:"lancamentos"`
	Dia         Dia                `json:"dia"`
}

func (f *ferramentas) registrar(ctx context.Context, _ *sdk.CallToolRequest, in entradaRegistro) (*sdk.CallToolResult, SaidaRegistro, error) {
	data, err := f.svc.LerData(in.Data)
	if err != nil {
		return nil, SaidaRegistro{}, err
	}
	reg := core.Registro{Data: data, Refeicao: in.Refeicao, Origem: core.OrigemAgente}
	for _, it := range in.Itens {
		reg.Itens = append(reg.Itens, core.Item{AlimentoID: it.AlimentoID, Gramas: it.Gramas, Estimado: it.Estimado})
	}
	ls, err := f.svc.Registrar(ctx, reg)
	if err != nil {
		return nil, SaidaRegistro{}, err
	}
	d, err := f.dia(ctx, data)
	return nil, SaidaRegistro{Lancamentos: ls, Dia: d}, err
}

type entradaEdicao struct {
	ID       int64    `json:"id" jsonschema:"id do lançamento (veja resumo_dia)"`
	Gramas   *float64 `json:"gramas,omitempty" jsonschema:"nova quantidade em gramas"`
	Refeicao *string  `json:"refeicao,omitempty" jsonschema:"cafe, almoco, lanche ou jantar"`
	Data     *string  `json:"data,omitempty" jsonschema:"nova data AAAA-MM-DD"`
}

type saidaLancamento struct {
	Lancamento store.Lancamento `json:"lancamento"`
}

func (f *ferramentas) editar(ctx context.Context, _ *sdk.CallToolRequest, in entradaEdicao) (*sdk.CallToolResult, saidaLancamento, error) {
	e := core.Edicao{Gramas: in.Gramas, Refeicao: in.Refeicao}
	if in.Data != nil {
		d, err := f.svc.LerData(*in.Data)
		if err != nil {
			return nil, saidaLancamento{}, err
		}
		e.Data = &d
	}
	l, err := f.svc.Editar(ctx, in.ID, e, core.OrigemAgente)
	return nil, saidaLancamento{Lancamento: l}, traduzir(err)
}

type entradaID struct {
	ID int64 `json:"id" jsonschema:"id do lançamento"`
}

func (f *ferramentas) remover(ctx context.Context, _ *sdk.CallToolRequest, in entradaID) (*sdk.CallToolResult, saidaLancamento, error) {
	l, err := f.svc.Remover(ctx, in.ID, core.OrigemAgente)
	return nil, saidaLancamento{Lancamento: l}, traduzir(err)
}

type saidaDesfazer struct {
	Revertido  string           `json:"revertido" jsonschema:"ação desfeita: criar, editar ou remover"`
	Lancamento store.Lancamento `json:"lancamento"`
}

func (f *ferramentas) desfazer(ctx context.Context, _ *sdk.CallToolRequest, _ struct{}) (*sdk.CallToolResult, saidaDesfazer, error) {
	d, err := f.svc.Desfazer(ctx, core.OrigemAgente)
	return nil, saidaDesfazer{Revertido: d.Acao, Lancamento: d.Lancamento}, err
}

type entradaPorcao struct {
	AlimentoID int64   `json:"alimento_id"`
	Nome       string  `json:"nome" jsonschema:"nome da medida, por exemplo: 1 concha"`
	Gramas     float64 `json:"gramas"`
}

func (f *ferramentas) definirPorcao(ctx context.Context, _ *sdk.CallToolRequest, in entradaPorcao) (*sdk.CallToolResult, store.Porcao, error) {
	p, err := f.svc.DefinirPorcao(ctx, in.AlimentoID, in.Nome, in.Gramas)
	return nil, p, traduzir(err)
}

type entradaMetas struct {
	Kcal float64 `json:"kcal"`
	Prot float64 `json:"prot_g"`
	Carb float64 `json:"carb_g"`
	Gord float64 `json:"gord_g"`
}

func (f *ferramentas) definirMetas(ctx context.Context, _ *sdk.CallToolRequest, in entradaMetas) (*sdk.CallToolResult, saidaMetas, error) {
	m, err := f.svc.DefinirMetas(ctx, in.Kcal, in.Prot, in.Carb, in.Gord)
	if err != nil {
		return nil, saidaMetas{}, err
	}
	return nil, saidaMetas{Definida: true, VigenteDesde: m.VigenteDesde, Meta: deMeta(m)}, nil
}

// traduzir deixa claro para o agente quando o id não existe.
func traduzir(err error) error {
	if errors.Is(err, core.ErrNaoEncontrado) {
		return errors.New("não encontrado: confira o id com resumo_dia ou buscar_alimento")
	}
	return err
}

// --- receitas e alimentos do usuário ---

type ingrediente struct {
	AlimentoID int64   `json:"alimento_id"`
	Nome       string  `json:"nome,omitempty"`
	Gramas     float64 `json:"gramas"`
	Kcal       float64 `json:"kcal,omitempty"`
}

// Receita como o agente vê.
type Receita struct {
	ID           int64         `json:"id"`
	Nome         string        `json:"nome"`
	Ingredientes []ingrediente `json:"ingredientes"`
	RendimentoG  float64       `json:"rendimento_g"`
	PesoPronto   bool          `json:"peso_pronto_informado" jsonschema:"falso se o rendimento é só a soma dos ingredientes"`
	NPorcoes     int           `json:"n_porcoes"`
	Porcao       store.Porcao  `json:"porcao"`
	PorPorcao    Nutrientes    `json:"por_porcao"`
	Por100g      Nutrientes    `json:"por_100g"`
}

func deReceita(rc core.Receita) Receita {
	out := Receita{ID: rc.ID, Nome: rc.Nome, RendimentoG: rc.RendimentoG, PesoPronto: rc.PesoInformado, NPorcoes: rc.NPorcoes,
		Porcao:    store.Porcao{AlimentoID: rc.ID, Nome: rc.NomePorcao, Gramas: rc.PorcaoG},
		PorPorcao: deMacros(rc.MacrosPorcao),
		Por100g:   Nutrientes{Kcal: rc.Kcal, Prot: rc.Prot, Carb: rc.Carb, Gord: rc.Gord, Fibra: rc.Fibra},
	}
	for _, it := range rc.Itens {
		out.Ingredientes = append(out.Ingredientes, ingrediente{AlimentoID: it.AlimentoID, Nome: it.AlimentoNome,
			Gramas: it.Gramas, Kcal: it.Macros.Kcal})
	}
	return out
}

func deMacros(m core.Macros) Nutrientes {
	return Nutrientes{Kcal: m.Kcal, Prot: m.Prot, Carb: m.Carb, Gord: m.Gord, Fibra: m.Fibra}
}

type saidaReceitas struct {
	Receitas []Receita `json:"receitas"`
}

func (f *ferramentas) listarReceitas(ctx context.Context, _ *sdk.CallToolRequest, _ struct{}) (*sdk.CallToolResult, saidaReceitas, error) {
	rcs, err := f.svc.Receitas(ctx)
	out := saidaReceitas{Receitas: []Receita{}}
	for _, rc := range rcs {
		out.Receitas = append(out.Receitas, deReceita(rc))
	}
	return nil, out, err
}

type entradaReceita struct {
	Nome         string        `json:"nome" jsonschema:"nome da receita, por exemplo: Marmita de frango com batata"`
	Ingredientes []ingrediente `json:"ingredientes" jsonschema:"alimento_id e gramas de cada ingrediente; nome e kcal são ignorados"`
	RendimentoG  float64       `json:"rendimento_g,omitempty" jsonschema:"peso pronto da panela; sem ele, vale a soma dos ingredientes"`
	NPorcoes     int           `json:"n_porcoes,omitempty" jsonschema:"em quantas porções rendeu (padrão 1)"`
	NomePorcao   string        `json:"nome_porcao,omitempty" jsonschema:"nome da porção, por exemplo: 1 marmita (padrão: 1 porção)"`
}

func (e entradaReceita) nova() core.NovaReceita {
	n := core.NovaReceita{Nome: e.Nome, RendimentoG: e.RendimentoG, NPorcoes: e.NPorcoes, NomePorcao: e.NomePorcao}
	for _, it := range e.Ingredientes {
		n.Itens = append(n.Itens, store.ItemReceita{AlimentoID: it.AlimentoID, Gramas: it.Gramas})
	}
	return n
}

func (f *ferramentas) criarReceita(ctx context.Context, _ *sdk.CallToolRequest, in entradaReceita) (*sdk.CallToolResult, Receita, error) {
	rc, err := f.svc.CriarReceita(ctx, in.nova())
	if err != nil {
		return nil, Receita{}, traduzir(err)
	}
	return nil, deReceita(rc), nil
}

type entradaEdicaoReceita struct {
	ID int64 `json:"id" jsonschema:"id da receita (veja listar_receitas)"`
	entradaReceita
}

func (f *ferramentas) editarReceita(ctx context.Context, _ *sdk.CallToolRequest, in entradaEdicaoReceita) (*sdk.CallToolResult, Receita, error) {
	rc, err := f.svc.EditarReceita(ctx, in.ID, in.nova())
	if err != nil {
		return nil, Receita{}, traduzir(err)
	}
	return nil, deReceita(rc), nil
}

type entradaAlimento struct {
	Nome      string     `json:"nome"`
	Categoria string     `json:"categoria,omitempty"`
	Por100g   Nutrientes `json:"por_100g" jsonschema:"valores por 100 g"`
}

func (f *ferramentas) criarAlimento(ctx context.Context, _ *sdk.CallToolRequest, in entradaAlimento) (*sdk.CallToolResult, Alimento, error) {
	a, err := f.svc.CriarAlimento(ctx, core.NovoAlimento{Nome: in.Nome, Categoria: in.Categoria,
		Por100g: core.Macros{Kcal: in.Por100g.Kcal, Prot: in.Por100g.Prot, Carb: in.Por100g.Carb, Gord: in.Por100g.Gord, Fibra: in.Por100g.Fibra}})
	if err != nil {
		return nil, Alimento{}, err
	}
	return nil, Alimento{ID: a.ID, Nome: a.Nome, Categoria: a.Categoria, Fonte: a.Fonte,
		Por100g: Nutrientes{Kcal: a.Kcal, Prot: a.Prot, Carb: a.Carb, Gord: a.Gord, Fibra: a.Fibra}}, nil
}
