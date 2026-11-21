from __future__ import annotations

from fastapi import Depends, FastAPI, Header, HTTPException, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse

from . import ingest, registry, retrieval
from .config import get_settings
from .db import jload
from .errors import INVALID_REQUEST, ProtocolError
from .models import (ChatRequest, ChatResponse, DocumentRegistration, IngestionAccepted, IngestionRequest,
                     IngestionStatus, SourceUpdate)

app = FastAPI(title="RAG 入库与问答", version="1.0")


@app.exception_handler(ProtocolError)
async def _protocol_error(_: Request, e: ProtocolError):
    return JSONResponse(status_code=e.http_status, content={"error": e.to_dict()})


@app.exception_handler(RequestValidationError)
async def _validation_error(_: Request, e: RequestValidationError):
    msg = "; ".join(f"{'.'.join(map(str, x['loc']))}: {x['msg']}" for x in e.errors())
    return JSONResponse(status_code=400,
                        content={"error": {"code": INVALID_REQUEST, "message": msg, "retryable": False}})


# ---------------- 鉴权 ----------------
def service_auth(authorization: str = Header(default="")) -> None:
    """内部接口使用服务身份鉴权。"""
    if not authorization.startswith("Bearer "):
        raise HTTPException(401, "缺少服务凭证")
    if authorization[7:] not in get_settings().tokens:
        raise HTTPException(403, "服务凭证无效")


def current_user(x_user_id: str = Header(default=""), x_user_roles: str = Header(default=""),
                 x_user_factory: str | None = Header(default=None),
                 x_user_workshop: str | None = Header(default=None)) -> registry.User:
    """用户身份由现有后端/网关鉴权后通过请求头传入。"""
    if not x_user_id:
        raise HTTPException(401, "未登录")
    roles = [r.strip() for r in x_user_roles.split(",") if r.strip()]
    return registry.User(x_user_id, x_user_factory, x_user_workshop, roles)


# ---------------- 5.1 / 5.2 内部导入接口 ----------------
def _status(job) -> IngestionStatus:
    return IngestionStatus(job_id=job["job_id"], document_id=job["document_id"], parse_id=job["parse_id"],
                           status=job["status"], indexed_chunk_count=job["indexed_chunk_count"],
                           updated_at=job["updated_at"], error=jload(job["error"]))


@app.post("/internal/v1/ingestions", status_code=202, response_model=IngestionAccepted,
          dependencies=[Depends(service_auth)])
def create_ingestion(req: IngestionRequest):
    job = ingest.submit(req)
    return IngestionAccepted(job_id=job["job_id"], parse_id=job["parse_id"], status=job["status"])


@app.get("/internal/v1/ingestions/{job_id}", response_model=IngestionStatus, dependencies=[Depends(service_auth)])
def get_ingestion(job_id: str):
    job = ingest.get_job(job_id)
    if job is None:
        raise HTTPException(404, "任务不存在")
    return _status(job)


# ---------------- 来源登记同步（后端 → RAG） ----------------
@app.post("/internal/v1/documents", status_code=204, dependencies=[Depends(service_auth)])
def register_document(body: DocumentRegistration):
    registry.upsert_document(**body.model_dump())


@app.post("/internal/v1/documents/{document_id}/revoke", status_code=204, dependencies=[Depends(service_auth)])
def revoke_document(document_id: str):
    registry.require_document(document_id)
    registry.revoke_document(document_id)


@app.patch("/internal/v1/sources/{source_id}", status_code=204, dependencies=[Depends(service_auth)])
def update_source(source_id: str, body: SourceUpdate):
    registry.update_source(source_id, **body.model_dump())


@app.post("/internal/v1/parses/{parse_id}/approve", status_code=204, dependencies=[Depends(service_auth)])
def approve_parse(parse_id: str):
    """quality=partial 人工复核通过；若该产物的入库任务因未审批失败，则重跑。"""
    registry.approve_parse(parse_id)
    ingest.retry(parse_id)


# ---------------- 问答 ----------------
@app.post("/api/v1/chat", response_model=ChatResponse)
def chat(body: ChatRequest, user: registry.User = Depends(current_user)):
    return retrieval.answer(user, body.question, body.top_k)
