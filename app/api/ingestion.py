from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException

from .. import ingest, registry
from ..db import jload
from ..models import (
    DocumentRegistration, IngestionAccepted, IngestionRequest, IngestionStatus, SourceUpdate,
)
from .auth import service_auth

router = APIRouter(prefix="/internal/v1", dependencies=[Depends(service_auth)])


@router.post("/ingestions", status_code=202, response_model=IngestionAccepted)
def create_ingestion(req: IngestionRequest):
    job = ingest.submit(req)
    return IngestionAccepted(job_id=job["job_id"], parse_id=job["parse_id"], status=job["status"])


@router.get("/ingestions/{job_id}", response_model=IngestionStatus)
def get_ingestion(job_id: str):
    job = ingest.get_job(job_id)
    if job is None:
        raise HTTPException(404, "任务不存在")
    return IngestionStatus(
        job_id=job["job_id"], document_id=job["document_id"], parse_id=job["parse_id"],
        status=job["status"], indexed_chunk_count=job["indexed_chunk_count"],
        updated_at=job["updated_at"], error=jload(job["error"]),
    )


@router.post("/documents", status_code=204)
def register_document(body: DocumentRegistration):
    registry.upsert_document(**body.model_dump())


@router.post("/documents/{document_id}/revoke", status_code=204)
def revoke_document(document_id: str):
    registry.require_document(document_id)
    registry.revoke_document(document_id)


@router.patch("/sources/{source_id}", status_code=204)
def update_source(source_id: str, body: SourceUpdate):
    registry.update_source(source_id, **body.model_dump())


@router.post("/parses/{parse_id}/approve", status_code=204)
def approve_parse(parse_id: str):
    registry.approve_parse(parse_id)
    ingest.retry(parse_id)
