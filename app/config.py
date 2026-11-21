from __future__ import annotations

from functools import lru_cache
from pathlib import Path

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="RAG_", env_file=".env", extra="ignore")

    data_dir: Path = Path("./data")
    # 解析方写入解析包的受控存储根目录；artifact_ref 相对此目录
    artifact_root: Path = Path("./data/storage")
    max_artifact_bytes: int = 50 * 1024 * 1024
    # 内部接口服务身份（逗号分隔多个）
    service_tokens: str = "parser-service-token-change-me"

    # Embedding：openai（OpenAI 兼容接口）| hash（离线，仅联调/测试）
    embedding_provider: str = "openai"
    embedding_model: str = "text-embedding-3-small"
    embedding_base_url: str | None = None
    embedding_api_key: str | None = None

    # 生成：openai | extractive（不调模型，直接返回摘录）
    llm_provider: str = "openai"
    llm_model: str = "gpt-4o-mini"
    llm_base_url: str | None = None
    llm_api_key: str | None = None

    chunk_max_chars: int = 600
    chunk_overlap_chars: int = 80
    top_k: int = 5
    index_max_retries: int = 3

    # 出处里的受控预览路径（由后端实现并按账号鉴权）
    preview_url_template: str = "/api/v1/documents/{document_id}/preview"

    @property
    def tokens(self) -> set[str]:
        return {t.strip() for t in self.service_tokens.split(",") if t.strip()}

    @property
    def db_path(self) -> Path:
        return self.data_dir / "rag.sqlite3"


@lru_cache
def get_settings() -> Settings:
    return Settings()
