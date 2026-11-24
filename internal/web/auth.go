package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	cookieSessao  = "rango_sessao"
	duracaoSessao = 30 * 24 * time.Hour
)

// sessao assina cookies com uma chave derivada da senha: trocar a senha derruba as sessões.
type sessao struct {
	senha []byte
	chave []byte
}

func novaSessao(senha string) *sessao {
	k := sha256.Sum256([]byte("rango-sessao:" + senha))
	return &sessao{senha: []byte(senha), chave: k[:]}
}

func (s *sessao) assinar(expira int64) string {
	m := hmac.New(sha256.New, s.chave)
	m.Write([]byte(strconv.FormatInt(expira, 10)))
	return hex.EncodeToString(m.Sum(nil))
}

func (s *sessao) emitir(w http.ResponseWriter) {
	expira := time.Now().Add(duracaoSessao)
	v := strconv.FormatInt(expira.Unix(), 10) + "." + s.assinar(expira.Unix())
	http.SetCookie(w, &http.Cookie{
		Name: cookieSessao, Value: v, Path: "/", Expires: expira,
		HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
}

func (s *sessao) valida(r *http.Request) bool {
	c, err := r.Cookie(cookieSessao)
	if err != nil {
		return false
	}
	exp, assinatura, ok := strings.Cut(c.Value, ".")
	if !ok {
		return false
	}
	n, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || time.Now().Unix() > n {
		return false
	}
	return hmac.Equal([]byte(assinatura), []byte(s.assinar(n)))
}

func (s *sessao) confere(senha string) bool {
	a := sha256.Sum256([]byte(senha))
	b := sha256.Sum256(s.senha)
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

func publico(path string) bool {
	return path == "/login" || path == "/sw.js" || path == "/manifest.webmanifest" ||
		strings.HasPrefix(path, "/static/")
}

func (s *Servidor) exigirLogin(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publico(r.URL.Path) || s.sessao.valida(r) {
			h.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/login")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

func (s *Servidor) paginaLogin(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "login", "pagina", map[string]any{"Titulo": "Entrar"})
}

func (s *Servidor) entrar(w http.ResponseWriter, r *http.Request) {
	if !s.sessao.confere(r.PostFormValue("senha")) {
		time.Sleep(time.Second) // freia tentativas em sequência
		s.render(w, http.StatusUnauthorized, "login", "pagina",
			map[string]any{"Titulo": "Entrar", "Erro": "Senha incorreta."})
		return
	}
	s.sessao.emitir(w)
	http.Redirect(w, r, "/diario", http.StatusSeeOther)
}

func (s *Servidor) sair(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: cookieSessao, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
