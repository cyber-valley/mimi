## ------------------------------- Builder Stage ------------------------------ ##
FROM python:3.13-bookworm AS builder

RUN apt-get update && apt-get install --no-install-recommends -y \
        build-essential && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

ADD https://astral.sh/uv/install.sh /install.sh
RUN chmod -R 655 /install.sh && /install.sh && rm /install.sh

ENV PATH="/root/.local/bin:$PATH"

WORKDIR /app

COPY ./pyproject.toml .

RUN uv sync

## ------------------------------- Production Stage ------------------------------ ##
FROM python:3.13-slim-bookworm AS production

WORKDIR /app

RUN apt-get install -y git

COPY /mimi mimi
COPY __main__.py .
COPY --from=builder /app/.venv .venv

ENV PATH="/app/.venv/bin:$PATH"

ENTRYPOINT ["python", "__main__.py"]
