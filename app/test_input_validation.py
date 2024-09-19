"""Unit tests for input_validation.py"""

import pytest

# Local imports
from constants import SUPPORTED_LANGUAGES, ExceptionMessages
from custom_exceptions import LanguageValidationError
from input_validation import validate_language


def test_validate_language_supported():
    """test validate_language with all supported languages"""
    # validate_language should never throw when fed a supported language
    # if it does this test will fail
    for lang in SUPPORTED_LANGUAGES:
        validate_language(lang)


def test_validate_language_unsupported():
    """test validate_language with an unsupported language"""
    lang = "FR"
    with pytest.raises(LanguageValidationError) as excinfo:
        validate_language(lang)
    assert ExceptionMessages.UNSUPPORTED_LANG.value == str(excinfo.value)
