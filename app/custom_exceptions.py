"""Collection of custom exceptions"""

from enum import Enum

from fastapi.exceptions import RequestValidationError


class CustomExceptionTypes(Enum):
    """Enumeration of all custom exception types to be update with each addition"""

    LANG = "value_error.str.language"


class LanguageValidationError(RequestValidationError):
    """To be raised when an invalid or non supported language is requested"""

    def __init__(self, msg: str, loc: list[str] | None = None):
        if loc is None:
            loc = ["body", "lang"]
        self.detail = build_error_detail(loc, CustomExceptionTypes.LANG.value, msg)
        super().__init__(self.detail)


def build_error_detail(loc: list[str], error_type: str, msg: str, ctx: dict | None = None):
    """Helper function to build the error body"""
    detail = {"loc": loc, "type": error_type, "msg": msg}
    if ctx:
        detail.update({"ctx": ctx})
    return [detail]
