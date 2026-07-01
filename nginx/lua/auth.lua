local cjson = require "cjson.safe"
local http = require "resty.http"
local resty_random = require "resty.random"
local resty_str = require "resty.string"

local _M = {}

local COOKIE_NAME = "auth_token"
local CACHE_TTL = 60

-- A well-formed W3C traceparent is 55 chars: "00-" + 32 hex (trace id) + "-" +
-- 16 hex (span id) + "-" + 2 hex (flags).
local TRACEPARENT_RE = "^00%-%x+%-%x+%-%x%x$"

-- ensure_traceparent returns the W3C traceparent for this request and sets it as
-- a request header so it is forwarded to the backend (which continues the trace).
-- nginx does not record its own span in this lightweight mode; the value gives
-- one trace id from the edge through the backend and into its logs.
--
-- Trust boundary: an inbound traceparent is only honored when the request
-- arrived through Cloudflare (cf-ray present) — otherwise any public client
-- could pin arbitrary trace ids (spoofing / cardinality attacks). All traffic
-- here is tunneled via Cloudflare, so a missing cf-ray means a client-supplied
-- traceparent is untrusted and we mint a fresh one. (For stronger assurance,
-- gate on a verified Cloudflare Access JWT instead of cf-ray presence.)
function _M.ensure_traceparent()
    local headers = ngx.req.get_headers()
    local tp = headers["traceparent"]
    local via_cloudflare = headers["cf-ray"] ~= nil

    if not (via_cloudflare and type(tp) == "string" and #tp == 55 and tp:match(TRACEPARENT_RE)) then
        local trace_bytes = resty_random.bytes(16)
        local span_bytes = resty_random.bytes(8)
        if trace_bytes and span_bytes then
            tp = "00-" .. resty_str.to_hex(trace_bytes) .. "-" .. resty_str.to_hex(span_bytes) .. "-01"
        else
            tp = nil
        end
    end
    if tp then
        ngx.req.set_header("traceparent", tp)
    end
    return tp
end

function _M.extract_token()
    local auth_header = ngx.req.get_headers()["Authorization"]
    if auth_header then
        local token = auth_header:match("^Bearer%s+(.+)$")
        if token and token ~= "" then
            return token
        end
    end

    local cookie_header = ngx.var.http_cookie
    if cookie_header then
        local token = cookie_header:match(COOKIE_NAME .. "=([^;]+)")
        if token and token ~= "" then
            return ngx.unescape_uri(token)
        end
    end

    return nil
end

function _M.validate_token(token, request_id, traceparent)
    local cache = ngx.shared.token_cache
    local cached = cache:get(token)
    if cached then
        local data = cjson.decode(cached)
        if data and data.user_id then
            return true, data
        end
    end

    local headers = {
        ["Authorization"] = "Bearer " .. token,
        ["X-Request-ID"] = request_id,
        ["X-Internal-Auth-Check"] = "1",
    }
    -- Propagate the trace so the auth-validate call is part of the request's
    -- trace rather than an orphan.
    if traceparent and traceparent ~= "" then
        headers["traceparent"] = traceparent
    end

    local httpc = http.new()
    httpc:set_timeout(3000)
    local res, err = httpc:request_uri("http://backend:8080/api/v1/auth/validate", {
        method = "GET",
        headers = headers,
        keepalive_timeout = 60000,
        keepalive_pool = 64,
    })

    if not res then
        ngx.log(ngx.ERR, "auth validate call failed: ", err)
        return false, nil, 500, { error = "Auth service unreachable" }
    end

    local data = cjson.decode(res.body or "")
    if res.status ~= 200 or not data or not data.valid or not data.user_id then
        return false, nil, 401, { error = "Invalid token" }
    end

    local payload = {
        user_id = data.user_id,
        username = data.username or "",
        role = data.role or "",
    }
    cache:set(token, cjson.encode(payload), CACHE_TTL)
    return true, payload, nil, nil
end

function _M.set_user_headers(data)
    ngx.req.set_header("X-User-ID", tostring(data.user_id))
    ngx.req.set_header("X-Username", data.username or "")
    if data.role and data.role ~= "" then
        ngx.req.set_header("X-User-Role", data.role)
    end
end

-- App routes that require a valid auth_token cookie (or Bearer) before serving the SPA.
function _M.requires_frontend_auth(uri)
    if uri == "/" then
        return true
    end
    if uri == "/search" or uri:find("^/search/") == 1 then
        return true
    end
    if uri == "/library" or uri:find("^/library/") == 1 then
        return true
    end
    if uri == "/requests" or uri:find("^/requests/") == 1 then
        return true
    end
    if uri == "/settings" or uri:find("^/settings/") == 1 then
        return true
    end
    return false
end

function _M.redirect_to_login(uri)
    local target = uri
    local qs = ngx.var.args
    if qs and qs ~= "" then
        target = target .. "?" .. qs
    end
    return ngx.redirect("/login?redirect=" .. ngx.escape_uri(target))
end

return _M
