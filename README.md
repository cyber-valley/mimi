# Mimi president

## Closed beta

### Problem

The rapidly developing Cyber Valley project has diverse sources of truth represented in the following resources:

- X.com tweets
- Telegram group chat messages
- Logseq knowledge base git repositories
- GitHub issues

Searching all of them becomes a time-consuming process and requires a simple way of querying all of them at one time.

### Solution

#### Announce

Develop RAG over all resources mentioned in the problem statement and provide an LLM-driven chat bot, which allows interactive and free-form querying of all of them at once.

#### Implementation details

##### Embedding model

We keep in mind that in the future it could be great to change the chosen model, but it requires complete recalculation for the whole dataset (because of different dimensions and algorithms in general). To handle this, we will store all source data "as is", so making embeddings will be a question of computation.
For the POC we will stick to the OpenAI [text-embedding-3-small](https://platform.openai.com/docs/guides/embeddings#embedding-models) which is pretty cheap and should work well enough.

![embedding-model-pricing](img/embedding-model-pricing.png)

##### LLM chat bot

Our solution is completely model-agnostic, so any provider could be used and switched on the fly.

##### Data store

We choose [Turso](https://docs.turso.tech/introduction) as our DBMS; it works perfectly with vector search, scales greatly on HDD drives, and has zero network latency because it's built on [libSQL](https://github.com/tursodatabase/libsql/).

##### Programming language

We will use Python & [LangChain](https://www.langchain.com/langchain) for the project because it'll just need glue between IO operations. Rust wouldn't make a visible difference in speed or durability and lacks ready-to-use packages for fast idea testing.

##### Parsing

###### X.com

We don't know for sure the general required number of accounts, their requirements, and their publicity. So for the start and completely for free, it's possible to use Google news RSS. As an example, here is the RSS feed generated for [@levelsio](https://x.com/levelsio) - https://news.google.com/rss/search?q=site:twitter.com/levelsio+when:7

###### Telegram groups

We offer to use the [Telegram Client API](https://core.telegram.org/#telegram-api). It requires its own Telegram account but in exchange has access to the whole history of messages (in super groups where it's allowed). The algorithm for adding support for a new group will be the same as adding a new participant to the group. Then we will download all message history (with a given threshold or fully), then listen to new messages and process them as well.

###### GitHub

We can use the [Webhooks API](https://docs.github.com/en/webhooks/webhook-events-and-payloads) to get updates on commits to the LogSeq files and issues.

### Feature improvements

- Embed media (pictures, video, and audio) as well
- Query and embed provided URLs in the text info
- Include URLs to the initial sources found with RAG
- Allow querying only given resources e.g., "What are the statuses of the current projects with aishift in GitHub issues"

## Minimal valuable product

### Problem

Straight forward RAG solution doesn't work good enough in case of awareness of sources and types of information, so queries about concrete messages in telegram groups or issues assigned to exact people and time boundaries don't work.

### Solution

Enrich documents metadata with all possible tags and implement additional filtering by them with LLM

#### Additional improvements / features

##### Self aware prompt

Make Mimi to know in general in what field of data it operates and what are it's responsibilities

##### Store chat history

Keep each user's conversation so Mimi will know about previous messages

##### Add GitHub project's board parsing

Pure GitHub issues scraping isn't enough, more information should be fetched from the API. TBD @MichaelBorisov

#### Implementation details

- Migrate from SQLite to [CozoDB](https://docs.cozodb.org/en/latest/index.html) for the easier metadata search and future improves (e.g. logseq document structure)
- Add context about CyberValley directly to the system prompt
- Store all chat history in Redis and take only fixed amount of messages to fit in the context window
- Improve GitHub scraper to parse more data
- Use LLM to extract required filters from customer's query and convert them into [Datalog](https://en.wikipedia.org/wiki/Datalog) query

## Version 1.2

### Telegram chat summarization

#### Problem

Dense messages that lack general context

#### Solution

The initial idea was to accumulate messages in a given time delta into a single document through a simple low-temperature prompt.
But it just decreases the density of data, and the initial problem still exists.
Based on the request (follow back, supply and etc), read all existing messages and extract info from them. Gemini Pro or Flash perfectly fits for zero cost.

### Chat history

#### Problem

Assistant isn't aware of the previous messages.

#### Solution

Migrate to PostgreSQL and store history in JSONB with a given limit of tokens. SQLite wouldn't work well because of parallel reads and writes (WAL mode wasn't tested yet).

Future improve - embed message history to allow access for any message like in ChatGPT chat.

### Logseg to cozo

#### Problem

The client wants this database.

#### Solution

Come up with a Logseg parser for proper graph building and migrate to cozo.

### Github board parsing

#### Problem

The current parser lacks info about the board and several other project's features.

#### Solution

Cover missing parts with the existing Github API.

### Past week summary

#### Problem

Time-consuming process of reading all updates and memorizing them.

#### Solution

Define key points of interest and summarize info into a report.

### Food ordering

#### Problem

I have no idea.

#### Solution

Examine requirements first.

### Daily follow up

#### Problem

Missing feedback from staff.

#### Solution

Generic and unique agents which get a history of each topic in Telegram and generate a follow-up message with mentions.

### Transaction import

#### Problem

Missing interface to query tickets' state.

#### Solution

Add an agent to the tickets' back-end which will query and aggregate any request about the contract state.
