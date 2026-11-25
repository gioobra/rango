// Chat com o Nutri: envia pelo htmx e acompanha a resposta por SSE.
(() => {
  const rolar = () => window.scrollTo({ top: document.body.scrollHeight, behavior: "smooth" });

  function travar(sim) {
    const form = document.getElementById("form-chat");
    if (!form) return;
    for (const el of form.elements) el.disabled = sim;
    if (!sim) form.elements.mensagem.focus();
  }

  // Só o negrito do markdown vira HTML, e só depois de escapar o texto (igual ao servidor).
  const escapar = (s) => s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&#34;", "'": "&#39;" })[c]);
  const negrito = (s) => escapar(s).replace(/\*\*([^*\n]+)\*\*/g, "<strong>$1</strong>");

  function bolha(tpl, texto) {
    const el = document.getElementById(tpl).content.firstElementChild.cloneNode(true);
    el.querySelector("[data-texto]").textContent = texto;
    return el;
  }

  function acompanhar(turno) {
    if (turno.dataset.acompanhando) return;
    turno.dataset.acompanhando = "1";
    travar(true);
    const pensando = turno.querySelector("[data-pensando]");
    const status = turno.querySelector("[data-status]");
    let atual = null;
    let bruto = "";
    let recebeu = false;
    const antes = (el) => { turno.insertBefore(el, pensando); recebeu = true; rolar(); };

    const es = new EventSource("/chat/turnos/" + turno.dataset.turno);
    es.addEventListener("texto", (e) => {
      if (!atual) { atual = bolha("tpl-nutri", ""); antes(atual); bruto = ""; }
      bruto += e.data;
      atual.querySelector("[data-texto]").innerHTML = negrito(bruto);
      status.textContent = "Escrevendo…";
      rolar();
    });
    es.addEventListener("ferramenta", (e) => {
      if (e.data) status.textContent = e.data;
    });
    es.addEventListener("cartao", (e) => {
      const caixa = document.createElement("div");
      caixa.innerHTML = e.data;
      const cartao = caixa.firstElementChild;
      if (!cartao) return;
      atual = null;
      antes(cartao);
      htmx.process(cartao);
    });
    es.addEventListener("erro", (e) => { atual = null; antes(bolha("tpl-erro", e.data)); });
    es.addEventListener("fim", () => {
      es.close();
      pensando.remove();
      travar(false);
      // O turno acabou antes de conectar: o histórico já está no servidor.
      if (!recebeu) location.reload();
    });
  }

  const procurar = () => document.querySelectorAll("[data-turno]").forEach(acompanhar);

  document.addEventListener("htmx:afterRequest", (e) => {
    if (e.target.id === "form-chat") {
      e.target.elements.atalho.value = "";
      if (e.detail.successful) e.target.reset();
    }
  });
  document.addEventListener("htmx:afterSwap", () => { procurar(); rolar(); });
  document.addEventListener("click", (e) => {
    const atalho = e.target.closest("[data-atalho], [data-comando]");
    const form = document.getElementById("form-chat");
    if (!atalho || !form || form.elements.mensagem.disabled) return;
    if (atalho.dataset.comando) form.elements.atalho.value = atalho.dataset.comando;
    else form.elements.mensagem.value = atalho.dataset.atalho;
    htmx.trigger(form, "submit");
  });
  procurar();
  rolar();
})();
