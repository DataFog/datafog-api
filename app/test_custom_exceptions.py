"""Unit tests for custom_exceptions.py"""

from custom_exceptions import (CustomExceptionTypes, LanguageValidationError,
                               build_error_detail)


def test_build_error_detail_with_ctx():
    loc = ["body", "text"]
    error_type = "test.error"
    msg = "test error message"
    ctx = {"limit_value": 10}
    result = build_error_detail(loc, error_type, msg, ctx)
    assert loc == result[0]["loc"], "loc not created correctly"
    assert error_type == result[0]["type"], "error type not created correctly"
    assert msg == result[0]["msg"], "message not created correctly"
    assert ctx == result[0]["ctx"], "context not created correctly"


def test_build_error_detail_no_ctx():
    loc = ["body", "text"]
    error_type = "test.error"
    msg = "test error message"
    result = build_error_detail(loc, error_type, msg)
    assert loc == result[0]["loc"], "loc not created correctly"
    assert error_type == result[0]["type"], "error type not created correctly"
    assert msg == result[0]["msg"], "message not created correctly"
    assert "ctx" not in result[0], "context field should not be assigned"


def test_template_error_init_with_loc():
    msg = "test error"
    loc = ["query"]
    test_error = LanguageValidationError(msg, loc)
    result = test_error.errors()
    assert LanguageValidationError == type(test_error), "error type mismatch"
    assert 1 == len(result), "error payload incorrect length"
    assert msg == result[0]["msg"], "error message not set correctly"
    assert loc == result[0]["loc"], "loc not set correctly"
    assert CustomExceptionTypes.LANG.value == result[0]["type"], "error type incorrect"
    assert "ctx" not in result[0], "context field should not be assigned"


def test_template_error_init_no_loc():
    msg = "test error"
    test_error = LanguageValidationError(msg)
    result = test_error.errors()
    assert LanguageValidationError == type(test_error), "error type mismatch"
    assert 1 == len(result), "error payload incorrect length"
    assert msg == result[0]["msg"], "error message not set correctly"
    assert ["body", "lang"] == result[0]["loc"], "loc not set correctly"
    assert CustomExceptionTypes.LANG.value == result[0]["type"], "error type incorrect"
    assert "ctx" not in result[0], "context field should not be assigned"
