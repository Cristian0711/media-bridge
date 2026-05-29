local cjson = require "cjson.safe"
local http = require "resty.http"

local _M = {}

local COOKIE_NAME = "auth_token"
local CACHE_TTL = 60

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

function _M.validate_token(token, request_id)
    local cache = ngx.shared.token_cache
    local cached = cache:get(token)
    if cached then
        local data = cjson.decode(cached)
        if data and data.user_id then
            return true, data
        end
    end

    local httpc = http.new()
    httpc:set_timeout(3000)
    local res, err = httpc:request_uri("http://backend:8080/api/v1/auth/validate", {
        method = "GET",
        headers = {
            ["Authorization"] = "Bearer " .. token,
            ["X-Request-ID"] = request_id,
            ["X-Internal-Auth-Check"] = "1",
        },
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
