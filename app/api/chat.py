from __future__ import annotations

from fastapi import APIRouter, Depends

from .. import registry, retrieval
from ..models import ChatRequest, ChatResponse
from .auth import current_user

router = APIRouter(prefix="/api/v1")


@router.post("/chat", response_model=ChatResponse)
def chat(body: ChatRequest, user: registry.User = Depends(current_user)):
    return retrieval.answer(user, body.question, body.top_k)
