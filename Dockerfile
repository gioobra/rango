# Imagem do Rango com o Nutri pela API (RANGO_LLM_MODEL). Não tem o claude: sem a API, a web sobe sem chat.
# O app.css vai versionado, então o build não precisa do Tailwind.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /rango ./cmd/rango && mkdir /data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /rango /rango
COPY --from=build --chown=nonroot:nonroot /data /data
ENV RANGO_DB=/data/rango.db \
    RANGO_ADDR=0.0.0.0:8080 \
    RANGO_PERMITIR_EXTERNO=1
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/rango"]
CMD ["serve"]
