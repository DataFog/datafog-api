"""Unit tests for authorization.py"""

import json
import secrets
from unittest.mock import mock_open, patch

import pytest
from authorization import is_valid_request, load_credentials_file
from constants import ExceptionMessages
from fastapi import HTTPException
from fastapi.security import HTTPBasicCredentials

TEST_USER = "jsmith"
TEST_PASSWORD = "1234"

@patch("authorization.load_valid_credentials")
def test_is_valid_request_true(mock_load_valid_credentials):
    # Define the mock and test data
    mock_data = (TEST_USER.encode("utf8"), TEST_PASSWORD.encode("utf8"))
    mock_load_valid_credentials.return_value = mock_data
    credentials = HTTPBasicCredentials(username=TEST_USER, password=TEST_PASSWORD)

    # Call the function
    result = is_valid_request(credentials)

    # Assert that the result is as expected
    assert result
    # Ensure that load_valid_credentials was called
    mock_load_valid_credentials.assert_called_once()


@patch("authorization.load_valid_credentials")
def test_is_valid_request_false_user(mock_load_valid_credentials):
    # Define the mock and test data
    mock_data = ("wrong_user".encode("utf8"), TEST_PASSWORD.encode("utf8"))
    mock_load_valid_credentials.return_value = mock_data
    credentials = HTTPBasicCredentials(username=TEST_USER, password=TEST_PASSWORD)

    # Call the function
    result = is_valid_request(credentials)

    # Assert that the result is as expected
    assert result is False
    # Ensure that load_valid_credentials was called
    mock_load_valid_credentials.assert_called_once()


@patch("authorization.load_valid_credentials")
def test_is_valid_request_false_password(mock_load_valid_credentials):
    # Define the mock and test data
    mock_data = (TEST_USER.encode("utf8"), "wrong_password".encode("utf8"))
    mock_load_valid_credentials.return_value = mock_data
    credentials = HTTPBasicCredentials(username=TEST_USER, password=TEST_PASSWORD)

    # Call the function
    result = is_valid_request(credentials)

    # Assert that the result is as expected
    assert result is False
    # Ensure that load_valid_credentials was called
    mock_load_valid_credentials.assert_called_once()


@patch("authorization.load_valid_credentials")
def test_is_valid_request_false_both(mock_load_valid_credentials):
    # Define the mock and test data
    mock_data = ("wrong_user".encode("utf8"), "wrong_password".encode("utf8"))
    mock_load_valid_credentials.return_value = mock_data
    credentials = HTTPBasicCredentials(username=TEST_USER, password=TEST_PASSWORD)

    # Call the function
    result = is_valid_request(credentials)

    # Assert that the result is as expected
    assert result is False
    # Ensure that load_valid_credentials was called
    mock_load_valid_credentials.assert_called_once()


def test_load_credentials_file():
    # Define the mock data
    mock_data = {"DATAFOG_AUTH_USER": TEST_USER, "DATAFOG_PASSWORD": TEST_PASSWORD}
    mock_file_content = json.dumps(mock_data)

    # Mock the open function and json.load
    with patch("builtins.open", mock_open(read_data=mock_file_content)), \
         patch("json.load", return_value=mock_data) as mock_json_load:

        # Call the function
        user, password = load_credentials_file()

        # Assert that the result is as expected
        assert secrets.compare_digest(user, TEST_USER.encode("utf8"))
        assert secrets.compare_digest(password, TEST_PASSWORD.encode("utf8"))

        # Ensure that json.load was called with the mock file object
        mock_json_load.assert_called_once()


def test_load_credentials_file_malformed_no_user_field():
    # Define the mock data
    mock_data = {"DATAFOG_PASSWORD": TEST_PASSWORD}
    mock_file_content = json.dumps(mock_data)

    with patch("builtins.open", mock_open(read_data=mock_file_content)), \
         patch("json.load", return_value=mock_data), \
         pytest.raises(HTTPException) as excinfo:
        load_credentials_file()
    assert ExceptionMessages.AUTH_USER_KEY.value == excinfo.value.detail


def test_load_credentials_file_malformed_no_password_field():
    # Define the mock data
    mock_data = {"DATAFOG_AUTH_USER": TEST_USER}
    mock_file_content = json.dumps(mock_data)

    with patch("builtins.open", mock_open(read_data=mock_file_content)), \
         patch("json.load", return_value=mock_data), \
         pytest.raises(HTTPException) as excinfo:
        load_credentials_file()
    assert ExceptionMessages.AUTH_PASS_KEY.value == excinfo.value.detail


# Run the test
if __name__ == "__main__":
    pytest.main()
