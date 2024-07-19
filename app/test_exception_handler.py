"""Unit tests for exception_handler.py"""
import json
from fastapi import status
from fastapi.exceptions import RequestValidationError
from exception_handler import exception_processor
from custom_exceptions import LanguageValidationError

REGEX_MSG = "string contains characters beyond Extended ASCII which are not supported"
REGEX_PATTERN = "Extended ASCII"

def test_exception_processor_status_code():
    exc = RequestValidationError([{"loc": ["body", "text"],
                                   "type": "value_error.str",
                                   "msg": "test error"}])
    result = exception_processor(None, exc)
    assert(status.HTTP_422_UNPROCESSABLE_ENTITY == result.status_code), "incorrect status code"

def test_exception_processor_regex_override():
    exc = RequestValidationError([{"loc": ["body", "text"],
                                   "type": "value_error.str.regex",
                                   "msg": "test error",
                                   "ctx": {"pattern": "^[\x00-\xFF]+$"}}])
    result = exception_processor(None, exc)
    msg = json.loads(result.body)["detail"][0]["msg"]
    pattern = json.loads(result.body)["detail"][0]["ctx"]["pattern"]
    assert(REGEX_MSG == msg), "regex error message wasn't overriden"
    assert(REGEX_PATTERN == pattern), "regex ctx-pattern wasn't overriden"

def test_exception_processor_non_regex_type_passthrough():
    exc = LanguageValidationError("test error message")
    result = exception_processor(None, exc)
    msg = json.loads(result.body)["detail"][0]["msg"]
    assert("test error message" == msg), "error message overriden incorrectly"
