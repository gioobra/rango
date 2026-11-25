package mcp

import (
	"context"
	"fmt"
	"math"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gioobra/rango/internal/core"
)

type entradaPeriodo struct {
	Dias int    `json:"dias,omitempty" jsonschema:"tamanho do período em dias (padrão 7, até 90)"`
	Fim  string `json:"fim,omitempty" jsonschema:"último dia, AAAA-MM-DD; vazio é hoje"`
}

type aderencia struct {
	DiasComMeta    int `json:"dias_com_meta"`
	DiasNaMetaKcal int `json:"dias_na_meta_kcal" jsonschema:"dias a até ±10% da meta de kcal"`
	DiasProteinaOK int `json:"dias_proteina_ok" jsonschema:"dias com proteína ≥ meta"`
}

type fatia struct {
	Refeicao  string  `json:"refeicao"`
	KcalMedia float64 `json:"kcal_media"`
	Pct       float64 `json:"pct_kcal"`
}

type itemRanking struct {
	AlimentoID int64   `json:"alimento_id"`
	Nome       string  `json:"nome"`
	Total      float64 `json:"total"`
	Vezes      int     `json:"vezes"`
}

type diaResumido struct {
	Data       string     `json:"data"`
	Registrado bool       `json:"registrado"`
	Totais     Nutrientes `json:"totais"`
}

type saidaAnalise struct {
	Inicio          string        `json:"inicio"`
	Fim             string        `json:"fim"`
	Dias            int           `json:"dias"`
	DiasRegistrados int           `json:"dias_registrados" jsonschema:"dias com ao menos um lançamento; só eles entram nas médias"`
	CoberturaPct    float64       `json:"cobertura_pct"`
	MediaDiaria     Nutrientes    `json:"media_diaria"`
	DesvioKcal      float64       `json:"desvio_padrao_kcal"`
	Distribuicao    Distribuicao  `json:"distribuicao_energia"`
	Meta            *Nutrientes   `json:"meta_atual,omitempty"`
	Aderencia       aderencia     `json:"aderencia"`
	PorRefeicao     []fatia       `json:"por_refeicao"`
	TopKcal         []itemRanking `json:"top_kcal" jsonschema:"alimentos que mais somaram kcal (total em kcal)"`
	TopProt         []itemRanking `json:"top_proteina" jsonschema:"alimentos que mais somaram proteína (total em g)"`
	PctEstimado     float64       `json:"pct_kcal_estimadas"`
	ProteinaPorKg   float64       `json:"proteina_g_por_kg,omitempty"`
	PorDia          []diaResumido `json:"por_dia"`
}

func ranking(r []core.ItemRanking) []itemRanking {
	out := []itemRanking{}
	for _, it := range r {
		out = append(out, itemRanking{AlimentoID: it.AlimentoID, Nome: it.Nome, Total: it.Total, Vezes: it.Vezes})
	}
	return out
}

func (f *ferramentas) analisePeriodo(ctx context.Context, _ *sdk.CallToolRequest, in entradaPeriodo) (*sdk.CallToolResult, saidaAnalise, error) {
	fim, err := f.svc.LerData(in.Fim)
	if err != nil {
		return nil, saidaAnalise{}, err
	}
	if in.Dias == 0 {
		in.Dias = 7
	}
	a, err := f.svc.AnalisePeriodo(ctx, fim, in.Dias)
	if err != nil {
		return nil, saidaAnalise{}, err
	}
	out := saidaAnalise{
		Inicio: a.Inicio.Format(core.FormatoData), Fim: a.Fim.Format(core.FormatoData),
		Dias: a.Dias, DiasRegistrados: a.DiasRegistrados, CoberturaPct: math.Round(a.Cobertura()*1000) / 10,
		MediaDiaria: deTotais(a.Media), DesvioKcal: a.DesvioKcal, Distribuicao: deDistribuicao(a.Distribuicao),
		Aderencia:   aderencia{DiasComMeta: a.DiasComMeta, DiasNaMetaKcal: a.DiasNaMetaKcal, DiasProteinaOK: a.DiasProteinaOK},
		PorRefeicao: []fatia{}, TopKcal: ranking(a.TopKcal), TopProt: ranking(a.TopProt),
		PctEstimado: a.PctEstimado, ProteinaPorKg: a.ProteinaPorKg,
	}
	if a.Meta != nil {
		out.Meta = deMeta(*a.Meta)
	}
	for _, r := range a.PorRefeicao {
		out.PorRefeicao = append(out.PorRefeicao, fatia{Refeicao: r.Chave, KcalMedia: r.KcalMedia, Pct: r.Pct})
	}
	for _, d := range a.PorDia {
		out.PorDia = append(out.PorDia, diaResumido{Data: d.Data.Format(core.FormatoData), Registrado: d.Registrado, Totais: deTotais(d.Totais)})
	}
	return nil, out, nil
}

type tdee struct {
	Kcal            float64 `json:"kcal"`
	MediaKcal       float64 `json:"media_kcal_comidas"`
	VariacaoKg      float64 `json:"variacao_tendencia_kg"`
	Dias            int     `json:"dias"`
	DiasRegistrados int     `json:"dias_registrados"`
}

type pontoPeso struct {
	Data      string   `json:"data"`
	Kg        *float64 `json:"kg,omitempty"`
	Tendencia float64  `json:"tendencia"`
}

type saidaPeso struct {
	UltimaData    string      `json:"ultima_pesagem_data,omitempty"`
	UltimaKg      float64     `json:"ultima_pesagem_kg,omitempty"`
	Tendencia     float64     `json:"tendencia_kg,omitempty"`
	VariacaoSem   float64     `json:"variacao_kg_por_semana"`
	TDEE          *tdee       `json:"tdee,omitempty"`
	FaltaParaTDEE string      `json:"falta_para_tdee,omitempty"`
	Serie         []pontoPeso `json:"serie"`
}

func (f *ferramentas) peso(ctx context.Context, _ *sdk.CallToolRequest, in entradaPeriodo) (*sdk.CallToolResult, saidaPeso, error) {
	fim, err := f.svc.LerData(in.Fim)
	if err != nil {
		return nil, saidaPeso{}, err
	}
	if in.Dias == 0 {
		in.Dias = 30
	}
	p, err := f.svc.Peso(ctx, fim, in.Dias)
	if err != nil {
		return nil, saidaPeso{}, err
	}
	out := saidaPeso{Tendencia: p.Tendencia, VariacaoSem: p.VariacaoSem, FaltaParaTDEE: p.FaltaParaTDEE, Serie: []pontoPeso{}}
	if p.Ultima != nil {
		out.UltimaData, out.UltimaKg = p.Ultima.Data, p.Ultima.Kg
	}
	if p.TemTDEE {
		out.TDEE = &tdee{Kcal: p.TDEE.Kcal, MediaKcal: p.TDEE.MediaKcal, VariacaoKg: p.TDEE.VariacaoKg,
			Dias: p.TDEE.Dias, DiasRegistrados: p.TDEE.DiasRegistrados}
	}
	for _, pt := range p.Serie {
		pp := pontoPeso{Data: pt.Data.Format(core.FormatoData), Tendencia: pt.Tendencia}
		if pt.Kg > 0 {
			kg := pt.Kg
			pp.Kg = &kg
		}
		out.Serie = append(out.Serie, pp)
	}
	return nil, out, nil
}

type entradaPeso struct {
	Kg   float64 `json:"kg"`
	Data string  `json:"data,omitempty" jsonschema:"AAAA-MM-DD; vazio é hoje"`
}

func (f *ferramentas) registrarPeso(ctx context.Context, req *sdk.CallToolRequest, in entradaPeso) (*sdk.CallToolResult, saidaPeso, error) {
	data, err := f.svc.LerData(in.Data)
	if err != nil {
		return nil, saidaPeso{}, err
	}
	if err := f.svc.RegistrarPeso(ctx, data, in.Kg); err != nil {
		return nil, saidaPeso{}, err
	}
	res, out, err := f.peso(ctx, req, entradaPeriodo{Dias: 7})
	return res, out, err
}

type entradaSimulacao struct {
	Data  string `json:"data,omitempty" jsonschema:"AAAA-MM-DD; vazio é hoje"`
	Itens []item `json:"itens"`
}

type itemSimulado struct {
	AlimentoID int64      `json:"alimento_id"`
	Nome       string     `json:"nome"`
	Gramas     float64    `json:"gramas"`
	Macros     Nutrientes `json:"macros"`
}

type saidaSimulacao struct {
	Itens        []itemSimulado `json:"itens"`
	Soma         Nutrientes     `json:"soma_dos_itens"`
	DiaDepois    Nutrientes     `json:"dia_depois"`
	Distribuicao Distribuicao   `json:"distribuicao_energia_dia_depois"`
	Restante     *Nutrientes    `json:"restante_depois,omitempty" jsonschema:"meta − dia depois; negativo é acima da meta"`
	NaMetaKcal   bool           `json:"dia_na_meta_kcal" jsonschema:"o dia fica a até ±10% da meta de kcal"`
}

func (f *ferramentas) simular(ctx context.Context, _ *sdk.CallToolRequest, in entradaSimulacao) (*sdk.CallToolResult, saidaSimulacao, error) {
	data, err := f.svc.LerData(in.Data)
	if err != nil {
		return nil, saidaSimulacao{}, err
	}
	var itens []core.Item
	for _, it := range in.Itens {
		itens = append(itens, core.Item{AlimentoID: it.AlimentoID, Gramas: it.Gramas})
	}
	sim, err := f.svc.Simular(ctx, data, itens)
	if err != nil {
		return nil, saidaSimulacao{}, traduzir(err)
	}
	out := saidaSimulacao{Soma: deTotais(sim.Soma), DiaDepois: deTotais(sim.DiaDepois),
		Distribuicao: deDistribuicao(sim.Distrib), NaMetaKcal: sim.MetaKcalOK}
	if sim.TemMeta {
		r := deTotais(sim.Restante)
		out.Restante = &r
	}
	for _, it := range sim.Itens {
		out.Itens = append(out.Itens, itemSimulado{AlimentoID: it.Alimento.ID, Nome: it.Alimento.Nome, Gramas: it.Gramas,
			Macros: Nutrientes{Kcal: it.Macros.Kcal, Prot: it.Macros.Prot, Carb: it.Macros.Carb, Gord: it.Macros.Gord, Fibra: it.Macros.Fibra}})
	}
	return nil, out, nil
}

// --- prompts ---

const promptRevisao = `Faça a minha revisão da semana.
1. Chame analise_periodo com dias = 7 e peso.
2. Comece pela cobertura: quantos dos 7 dias têm registro. Se for menos de 5, avise que as médias valem pouco.
3. Compare a média diária com a meta (kcal e proteína) e diga a aderência: dias na meta de kcal e dias que bateram a proteína.
4. Comente a distribuição de energia, a fibra média, o peso de cada refeição e os alimentos que mais pesaram.
5. Se houver TDEE, compare com a meta de kcal e diga o que isso sugere; se não houver, diga o que falta.
6. Termine com no máximo 3 ajustes práticos para a próxima semana.
Use só os números das ferramentas, sem recalcular nada.`

const promptSugerir = `Sugira o que eu como %s para fechar a meta de hoje.
1. Veja resumo_dia para saber o que falta (restante).
2. Monte 2 ou 3 opções simples com alimentos de buscar_alimento ou das minhas receitas (listar_receitas).
3. Para cada opção, use simular_refeicao e ajuste os gramas até o dia ficar perto da meta, priorizando a proteína.
4. Mostre cada opção com as quantidades e o resultado que a simulação devolveu. Não registre nada.`

func adicionarPrompts(s *sdk.Server) {
	s.AddPrompt(&sdk.Prompt{Name: "revisao_semanal", Title: "Revisão da semana",
		Description: "Revisão dos últimos 7 dias com os números calculados pelo Rango."},
		func(context.Context, *sdk.GetPromptRequest) (*sdk.GetPromptResult, error) {
			return texto(promptRevisao), nil
		})
	s.AddPrompt(&sdk.Prompt{Name: "sugerir_refeicao", Title: "O que eu como?",
		Description: "Sugestões para fechar a meta do dia, com quantidades simuladas.",
		Arguments:   []*sdk.PromptArgument{{Name: "refeicao", Description: "cafe, almoco, lanche ou jantar (opcional)"}}},
		func(_ context.Context, req *sdk.GetPromptRequest) (*sdk.GetPromptResult, error) {
			quando := "agora"
			if r := req.Params.Arguments["refeicao"]; r != "" {
				quando = "no " + strings.ToLower(core.NomeRefeicao(r))
			}
			return texto(fmt.Sprintf(promptSugerir, quando)), nil
		})
}

func texto(t string) *sdk.GetPromptResult {
	return &sdk.GetPromptResult{Messages: []*sdk.PromptMessage{{Role: "user", Content: &sdk.TextContent{Text: t}}}}
}
