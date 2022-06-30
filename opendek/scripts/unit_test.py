#!/usr/bin/env python3

# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Intel Corporation
"Provides unit tests for scripts"

import unittest

class TestConfig(unittest.TestCase):
    "For testing json config"
    SCHEMA = {
        "$schema": "http://json-schema.org/draft-07/schema#",
        "type": "object",
        "properties": {
            "name": { "$ref": "#/$defs/hostname"}
        }
    }
    CONFIG_PATH = "deploy_esp/config_schema.json"

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        import jsonschema
        import json
        with open(self.CONFIG_PATH, "r", encoding="utf-8") as f:
            defs = json.load(f)["$defs"]
        self.SCHEMA["$defs"] = defs
        self.validator = jsonschema.Draft7Validator(self.SCHEMA,
            format_checker=jsonschema.draft7_format_checker)


    def validate_host(self, hostname):
        "Validates hostname using class schema"
        test = {"name": hostname}
        return self.validator.is_valid(test)

    def test_valid_hostnames(self):
        "Test if set of hostnames is valid"
        valid = [
            "sub.domain.example",
            "sub-domain.example",
            "sub.domain-example",
            "sub-domain-example",
            "mobica.pl",
            "hostname",
            "host.name",
            "host-name",
            "host--name",
            "01.org",
            "h0st",
            "host1",
            "snilv-02ms.com",
            "snilv---test",
            "master",
            "bazyli.snilp-03.ue-pl",
            "aa"]
        for v in valid:
            self.assertTrue(self.validate_host(v),
                f"Hostname '{v}' should be checked as valid hostname")

    def test_invalid_hostnames(self):
        "Test if set of hostnames is invalid"
        invalid = [
            "Test",
            "TEST.com",
            ".domain.com",
            "domain.com.",
            "unamE",
            "-host.snilp",
            "Host1",
            "host..name",
            "hostname-",
            "hostname.",
            "test-.com",
            "-hostname",
            ".hostname",
            "a",
            "a" * 61 + ".pl"]
        for v in invalid:
            self.assertFalse(self.validate_host(v),
                f"Hostname '{v}' should be checked as invalid hostname")

if __name__ == '__main__':
    unittest.main()
