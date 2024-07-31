"""Unit tests for authorization.py"""

# Standard library imports
from unittest.mock import patch

import pytest

# Third party imports
from fastapi import HTTPException, status
from fastapi.security import HTTPBasicCredentials

# Local imports
from authorization import (
    get_authorization,
    get_required_authorization_type,
    is_valid_basic_request,
    is_valid_request,
    load_valid_credentials,
)
from constants import PASSWORD_KEY, USER_KEY, AuthTypes, ExceptionMessages

TEST_USER = "jsmith"
TEST_PASSWORD = "1234"


@patch("os.getenv")
@patch("authorization.AUTH_ENABLED", True)
def test_get_required_authorization_type_invalid_type(mock_getenv):
    # Define the mock return
    mock_getenv.return_value = "fake_type"

    result = get_required_authorization_type()

    assert result == AuthTypes.NO_AUTH
    mock_getenv.assert_called_once()


@patch("authorization.is_valid_request")
@patch("authorization.AUTH_ENABLED", True)
@patch("authorization.ACTIVE_AUTH_TYPE", AuthTypes.HTTP_BASIC)
def test_get_authorization_granted(mock_is_valid_request):
    # Define the mock return
    mock_is_valid_request.return_value = True

    result = get_authorization(None)

    assert result == AuthTypes.HTTP_BASIC
    mock_is_valid_request.assert_called_once()


@patch("authorization.is_valid_request")
@patch("authorization.AUTH_ENABLED", False)
@patch("authorization.ACTIVE_AUTH_TYPE", AuthTypes.NO_AUTH)
def test_get_authorization_no_auth(mock_is_valid_request):
    result = get_authorization(None)

    assert result == AuthTypes.NO_AUTH
    mock_is_valid_request.assert_not_called()


@patch("authorization.is_valid_request")
@patch("authorization.AUTH_ENABLED", True)
@patch("authorization.ACTIVE_AUTH_TYPE", AuthTypes.HTTP_BASIC)
def test_get_authorization_rejected(mock_is_valid_request):
    # Define the mock return
    mock_is_valid_request.return_value = False

    with pytest.raises(HTTPException) as exc_info:
        get_authorization(None)

    assert exc_info.value.status_code == status.HTTP_401_UNAUTHORIZED
    assert exc_info.value.detail == ExceptionMessages.UNAUTHORIZED.value
    mock_is_valid_request.assert_called_once()


@patch("authorization.is_valid_basic_request")
@patch("authorization.ACTIVE_AUTH_TYPE", AuthTypes.HTTP_BASIC)
def test_is_valid_request_true(mock_is_valid_basic_request):
    # Define the mock and test data
    mock_is_valid_basic_request.return_value = True

    # Call the function
    result = is_valid_request(None)

    # Assert that the result is as expected
    assert result
    # Ensure that mocks were called
    mock_is_valid_basic_request.assert_called_once()


@patch("authorization.is_valid_basic_request")
@patch("authorization.ACTIVE_AUTH_TYPE", AuthTypes.HTTP_BASIC)
def test_is_valid_request_false(mock_is_valid_basic_request):
    # Define the mock and test data
    mock_is_valid_basic_request.return_value = False

    # Call the function
    result = is_valid_request(None)

    # Assert that the result is as expected
    assert result is False
    # Ensure that mocks were called
    mock_is_valid_basic_request.assert_called_once()


@patch("authorization.is_valid_basic_request")
@patch("authorization.ACTIVE_AUTH_TYPE", AuthTypes.NO_AUTH)
def test_is_valid_request_not_basic_auth(mock_is_valid_basic_request):
    # Define the mock and test data
    mock_is_valid_basic_request.return_value = False

    # Call the function
    result = is_valid_request(None)

    # Assert that the result is as expected
    assert result is False
    # Ensure that mocks were not called
    mock_is_valid_basic_request.assert_not_called()


@patch("authorization.load_valid_credentials")
def test_is_valid_basic_request_true(mock_load_valid_credentials):
    # Define the mock and test data
    mock_data = (TEST_USER.encode("utf8"), TEST_PASSWORD.encode("utf8"))
    mock_load_valid_credentials.return_value = mock_data
    credentials = HTTPBasicCredentials(username=TEST_USER, password=TEST_PASSWORD)

    # Call the function
    result = is_valid_basic_request(credentials)

    # Assert that the result is as expected
    assert result
    # Ensure that load_valid_credentials was called
    mock_load_valid_credentials.assert_called_once()


@patch("authorization.load_valid_credentials")
def test_is_valid_basic_request_false_user(mock_load_valid_credentials):
    # Define the mock and test data
    mock_data = ("wrong_user".encode("utf8"), TEST_PASSWORD.encode("utf8"))
    mock_load_valid_credentials.return_value = mock_data
    credentials = HTTPBasicCredentials(username=TEST_USER, password=TEST_PASSWORD)

    # Call the function
    result = is_valid_basic_request(credentials)

    # Assert that the result is as expected
    assert result is False
    # Ensure that load_valid_credentials was called
    mock_load_valid_credentials.assert_called_once()


@patch("authorization.load_valid_credentials")
def test_is_valid_basic_request_false_password(mock_load_valid_credentials):
    # Define the mock and test data
    mock_data = (TEST_USER.encode("utf8"), "wrong_password".encode("utf8"))
    mock_load_valid_credentials.return_value = mock_data
    credentials = HTTPBasicCredentials(username=TEST_USER, password=TEST_PASSWORD)

    # Call the function
    result = is_valid_basic_request(credentials)

    # Assert that the result is as expected
    assert result is False
    # Ensure that load_valid_credentials was called
    mock_load_valid_credentials.assert_called_once()


@patch("authorization.load_valid_credentials")
def test_is_valid_basic_request_false_both(mock_load_valid_credentials):
    # Define the mock and test data
    mock_data = ("wrong_user".encode("utf8"), "wrong_password".encode("utf8"))
    mock_load_valid_credentials.return_value = mock_data
    credentials = HTTPBasicCredentials(username=TEST_USER, password=TEST_PASSWORD)

    # Call the function
    result = is_valid_basic_request(credentials)

    # Assert that the result is as expected
    assert result is False
    # Ensure that load_valid_credentials was called
    mock_load_valid_credentials.assert_called_once()


@patch.dict("os.environ", {USER_KEY: "test_user", PASSWORD_KEY: "test_pass"})
def test_load_valid_credentials_success():
    result = load_valid_credentials()
    assert result[0] == "test_user".encode("utf8")
    assert result[1] == "test_pass".encode("utf8")


@patch.dict("os.environ", {PASSWORD_KEY: "test_pass"}, clear=True)
def test_load_valid_credentials_no_user():
    with pytest.raises(HTTPException) as exc_info:
        load_valid_credentials()

    assert exc_info.value.status_code == status.HTTP_401_UNAUTHORIZED
    assert exc_info.value.detail == ExceptionMessages.AUTH_USER_KEY.value


@patch.dict("os.environ", {USER_KEY: "test_user"}, clear=True)
def test_load_valid_credentials_no_password():
    with pytest.raises(HTTPException) as exc_info:
        load_valid_credentials()

    assert exc_info.value.status_code == status.HTTP_401_UNAUTHORIZED
    assert exc_info.value.detail == ExceptionMessages.AUTH_USER_KEY.value
