from __future__ import annotations

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse

from .api import chat, ingestion
from .errors import INVALID_REQUEST, ProtocolError

app = FastAPI(title="RAG 入库与问答", version="1.0")
app.include_router(ingestion.router)
app.include_router(chat.router)


@app.exception_handler(ProtocolError)
async def _protocol_error(_: Request, error: ProtocolError):
    return JSONResponse(status_code=error.http_status, content={"error": error.to_dict()})


@app.exception_handler(RequestValidationError)
async def _validation_error(_: Request, error: RequestValidationError):
    message = "; ".join(f"{'.'.join(map(str, entry['loc']))}: {entry['msg']}" for entry in error.errors())
    return JSONResponse(
        status_code=400,
        content={"error": {"code": INVALID_REQUEST, "message": message, "retryable": False}},
    )
