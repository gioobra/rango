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
