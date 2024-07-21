"""Unit tests for input_validation.py"""

import pytest
from custom_exceptions import LanguageValidationError
from input_validation import validate_language


def test_validate_language_supported():
    lang = "EN"
    try:
        validate_language(lang)
    except LanguageValidationError as e:
        pytest.fail(f"validate_language raised {e} unexpectedly when provided {lang}")


def test_validate_language_unsupported():
    lang = "FR"
    with pytest.raises(LanguageValidationError) as excinfo:
        validate_language(lang)
    assert (
        "Unsupported language request, please try a language listed in the DataFog docs"
        == str(excinfo.value)
    )
