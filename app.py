import os
import time
import random
import json
import zipfile
import tempfile
from flask import Flask, request, jsonify
from selenium import webdriver
from selenium.webdriver.chrome.service import Service
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from webdriver_manager.chrome import ChromeDriverManager

app = Flask(__name__)
MAX_PRICE = 15.0

def create_proxy_extension(proxy):
    if not proxy or '@' not in proxy: return None
    p = proxy.replace("http://", "").replace("https://", "")
    user_pass, host_port = p.split('@')
    user, pwd = user_pass.split(':')
    host, port = host_port.split(':')
    
    manifest_json = """
    {
        "version": "1.0.0", "manifest_version": 2, "name": "Chrome Proxy",
        "permissions": ["proxy", "tabs", "unlimitedStorage", "storage", "<all_urls>", "webRequest", "webRequestBlocking"],
        "background": { "scripts": ["background.js"] }, "minimum_chrome_version": "22.0.0"
    }
    """
    background_js = """
    var config = { mode: "fixed_servers", rules: { singleProxy: { scheme: "http", host: "%s", port: parseInt("%s") }, bypassList: ["localhost"] } };
    chrome.proxy.settings.set({value: config, scope: "regular"}, function() {});
    function callbackFn(details) { return { authCredentials: { username: "%s", password: "%s" } }; }
    chrome.webRequest.onAuthRequired.addListener(callbackFn, {urls: ["<all_urls>"]}, ['blocking']);
    """ % (host, port, user, pwd)

    plugin_file = tempfile.NamedTemporaryFile(delete=False, suffix='.zip')
    with zipfile.ZipFile(plugin_file.name, 'w') as zp:
        zp.writestr("manifest.json", manifest_json)
        zp.writestr("background.js", background_js)
    return plugin_file.name

def setup_driver(proxy=None):
    chrome_options = Options()
    chrome_options.add_argument("--no-sandbox")
    chrome_options.add_argument("--disable-dev-shm-usage")
    chrome_options.add_argument("--disable-gpu")
    chrome_options.add_argument("--window-size=1920,1080")
    
    # إعدادات هامة لـ GitHub Actions
    chrome_options.add_argument("--headless=new")
    chrome_options.add_argument("--disable-blink-features=AutomationControlled")
    chrome_options.add_experimental_option("excludeSwitches", ["enable-automation"])
    chrome_options.add_experimental_option('useAutomationExtension', False)

    if proxy:
        ext_file = create_proxy_extension(proxy)
        if ext_file: 
            chrome_options.add_extension(ext_file)

    driver = webdriver.Chrome(service=Service(ChromeDriverManager().install()), options=chrome_options)
    driver.execute_script("Object.defineProperty(navigator, 'webdriver', {get: () => undefined})")
    return driver

# ... (باقي دوال الفحص get_shopify_product و shopify_checkout_real كما هي بدون تغيير) ...

@app.route('/shopify', methods=['GET'])
def shopify_endpoint():
    site = request.args.get('site')
    cc = request.args.get('cc')
    proxy = request.args.get('proxy')

    if not site or not cc:
        return jsonify({"Response": "Missing Parameters"})

    try:
        driver = setup_driver(proxy=proxy)
        result = shopify_checkout_real(site, cc, driver)
        return jsonify(result)
    except Exception as e:
        return jsonify({"Response": f"Setup Error: {str(e)}"})

if __name__ == '__main__':
    # GitHub Actions يحتاج إلى تشغيل السيرفر على المنفذ 5000
    app.run(host='0.0.0.0', port=5000)
