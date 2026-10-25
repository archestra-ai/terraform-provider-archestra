import copy
import importlib.util
import json
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("marketplace", Path(__file__).with_name("verify-marketplace.py"))
marketplace = importlib.util.module_from_spec(spec)
spec.loader.exec_module(marketplace)


class PublicMarketplaceTests(unittest.TestCase):
    version = "v3.0.3"
    digest = "sha256:" + "a" * 64
    icon = b'<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32"><path d="M0 0h32v32H0z"/></svg>'

    def page(self):
        return {
            "query": {"packageType": "providers", "packageAccount": "archestra", "packageName": "provider-archestra", "packageVersion": self.version},
            "props": {"pageProps": {"dehydratedState": {"queries": [
                {"queryKey": ["packageMetadata", "archestra", "provider-archestra", self.version], "state": {"data": {
                    "public": True, "repoKey": "archestra/provider-archestra", "type": "provider", "digest": self.digest}}},
                {"queryKey": ["packageVersions", "archestra", "provider-archestra"], "state": {"data": {
                    "versions": [{"subscription_version": self.version}]}}},
            ]}}},
        }

    def check(self, page, icon=None):
        html = ('<script id="__NEXT_DATA__" type="application/json">' + json.dumps(page) + '</script>').encode()
        return marketplace.verify(html, icon or self.icon, self.version, self.digest, self.icon)

    def test_exact_release_and_logo_pass_with_marketplace_resizing(self):
        resized = self.icon.replace(b'width="32" height="32"', b'width="150" height="150"')
        self.assertEqual(self.check(self.page(), resized)["digest"], self.digest)

    def test_generic_spa_wrong_version_private_stale_digest_and_placeholder_fail(self):
        original = self.page()
        for mutate in (
            lambda p: p["query"].update(packageVersion="v3.0.0"),
            lambda p: p["props"]["pageProps"]["dehydratedState"]["queries"][0]["state"]["data"].update(public=False),
            lambda p: p["props"]["pageProps"]["dehydratedState"]["queries"][0]["state"]["data"].update(digest="sha256:old"),
            lambda p: p["props"]["pageProps"]["dehydratedState"]["queries"][1]["state"]["data"].update(versions=[]),
        ):
            changed = copy.deepcopy(original)
            mutate(changed)
            with self.assertRaises(ValueError):
                self.check(changed)
        with self.assertRaises(ValueError):
            marketplace.verify(b'<html>Sign in</html>', self.icon, self.version, self.digest, self.icon)
        with self.assertRaises(ValueError):
            self.check(original, self.icon.replace(b'M0 0h32v32H0z', b'M1 1h30v30H1z'))

    def test_official_icon_is_square_svg_without_external_content(self):
        root = marketplace.ET.fromstring(marketplace.ICON.read_bytes())
        self.assertEqual(root.attrib["width"], root.attrib["height"])
        self.assertEqual(root.attrib["viewBox"].split()[2], root.attrib["viewBox"].split()[3])
        self.assertTrue(list(root))
        self.assertTrue(all(node.tag == "{http://www.w3.org/2000/svg}path" for node in root))
        self.assertTrue(all(set(node.attrib) <= {"d", "fill"} for node in root))


if __name__ == "__main__":
    unittest.main()
