from __future__ import annotations


class ProtocolError(Exception):
    """带协议错误码的异常，对应 {"code","message","retryable"}。"""

    def __init__(self, code: str, message: str, retryable: bool = False, http_status: int = 400):
        super().__init__(message)
        self.code, self.message, self.retryable, self.http_status = code, message, retryable, http_status

    def to_dict(self) -> dict:
        return {"code": self.code, "message": self.message, "retryable": self.retryable}


ARTIFACT_INVALID = "ARTIFACT_INVALID"
SOURCE_NOT_APPROVED = "SOURCE_NOT_APPROVED"
INDEX_UNAVAILABLE = "INDEX_UNAVAILABLE"
PARSE_CONFLICT = "PARSE_CONFLICT"
DOCUMENT_NOT_FOUND = "DOCUMENT_NOT_FOUND"
INVALID_REQUEST = "INVALID_REQUEST"
