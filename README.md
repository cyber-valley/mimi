# Mimi

Mimi is an LLM-driven, multi-source assistant designed to unify and enrich knowledge aggregation for Cyber Valley and similar dynamic projects. It automates retrieval-augmented generation (RAG) and enables natural language interaction with diverse sources such as Telegram, Logseq, and GitHub.

---

## Services

- **LLM-powered Chatbot**  
  Telegram agent with context-aware chat, topic following, metadata filtering, and periodic summarization.
  Model-agnostic: uses OpenRouter or Gemini (via Genkit).
- **Scrapers & Data Synchronization**  
  - **Logseq**: Git repo-based knowledge base, parsed & indexed to CozoDB.
  - **GitHub**: Watches events/issues/boards and synchronizes project data.
  - **Telegram**: Ingests group/forum messages using Telegram Client API.
- **RAG Engine**  
  Stores and retrieves relevant content from all sources; operates over structured and text data.
- **Summarization Agents**  
  Generate daily, weekly, and topic-based reports.

---

## Technologies & Architecture

- **Codebase:** Go 1.24
- **LLM Integrations:** [OpenRouter](https://openrouter.ai/), [OpenAI](https://platform.openai.com/), [Gemini](https://ai.google.dev/gemini-api), via [Genkit](https://github.com/firebase/genkit)
- **Vector Search & DB:** Hybrid  
  - **Metadata, settings, chat:** PostgreSQL (via `pgx`)
  - **Graph/semantic queries:** [CozoDB](https://github.com/cozodb/cozo-lib-go)
- **Containerization:** Podman/Docker/OCI
- **Configuration:** `.env` / `example.env`, `mimi_config.json`
- **Infrastructure-as-Code:** Ansible (services, Postgres container, config)

---

## Key Design Decisions

- **Model/Provider Agnostic:** Easily swap LLMs or embedding providers via config/environment.
- **Source-Agnostic RAG:** All documents from Telegram/Logseq/GitHub are stored raw for future (re-)embedding.
- **Metadata Enrichment:** Structured metadata/tagging at ingest for granular filtering.
- **Full Conversation Memory:** Chat state/history stored as JSONB in Postgres; selective context windowing.
- **Logseq Native Graph:** Implements own Logseq parser/graph sync and custom Datalog-like querying.
- **Full Infrastructure Automation:** Use Makefile, Ansible for build/run/deploy/migrate.

---

## Usage

### Development

- **Build:**  
  `make install`, `make dev-db`, `make migrate-up` `make run` (start bot & scrapers)
- **Format/Lint/Test:**  
  `make format`, `make vet`, `make test`
- **Migrations:**  
  `make migrate-up` / `make migrate-down` (Geni, SQLC, etc.)
- **Deployment:**
  `make -C ansible/ deploy-service`

### Deployment

- **Container:**  
  `podman build -t mimi .`
- **Env/config:**  
  Configure all relevant API keys and DB params in `.env`/`example.env`
- **Ansible:**  
  - `ansible-playbook ansible/postgres.yml` (start DB)
  - `ansible-playbook ansible/server.yml` (install deps)
  - `ansible-playbook ansible/service.yml` (build/deploy service)

---

## Source Overview

- `cmd/app/` — main entrypoint (Telegram bot, orchestration)
- `cmd/scraper/{github,logseq,telegram}/` — resource-specific sync services (mostly for the testing)
- `prompts/` — system/user prompts for RAG and LLMs
- `internal/bot/` — bot logic, context, LLM/pluggable agents
- `internal/provider/{github,logseq,telegram}/` — data adapters, scraping, parsing
- `internal/persist/` — Auto generates sqlc queries from [sql/queries](sql/queries)
- `ansible/` — automation for DB, network, service deployment

---

## Maintained Technologies

- Go, Genkit, OpenRouter, OpenAI, Gemini
- PostgreSQL, CozoDB
- Podman, Ansible, Make, SQLC, Geni
- Telegram Client/HTTP APIs, GitHub APIs, Logseq

---

## Example `.env` variables

See `example.env`

---

## See Also

- [Makefile](Makefile)
- [Containerfile](Containerfile)
- [ansible/](ansible/) — Playbooks for infra

---

**Status:** Alpha · Multi-source RAG platform
