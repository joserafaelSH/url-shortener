-- Token bucket rate limiter, executado atomicamente dentro do Redis.
--
-- KEYS[1] = chave do hash que representa o bucket (ex: "bucket:user1")
-- ARGV[1] = maxTokens    (capacidade máxima do bucket)
-- ARGV[2] = refillRate   (tokens repostos por segundo)
-- ARGV[3] = requestCost  (quantos tokens este request consome)
-- ARGV[4] = now          (timestamp atual em segundos, fornecido pelo cliente Go)
--
-- Retorna 1 se o request foi permitido, 0 se foi negado.

local maxTokens = tonumber(ARGV[1])
local refillRate = tonumber(ARGV[2])
local requestCost = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

-- HMGET busca os dois campos de uma vez. Se a chave não existir, o Redis
-- devolve `false` (não `nil`) para cada campo ausente -- convenção do
-- Redis-Lua, diferente do Lua puro.
local vals = redis.call('HMGET', KEYS[1], 'tokens', 'timestamp')

local currentTokens
local lastTimestamp

if vals[1] == false then
    -- bucket novo: começa cheio, igual você decidiu na versão in-memory
    currentTokens = maxTokens
    lastTimestamp = now
else
    -- tudo que vem do Redis chega como STRING, mesmo números -- por isso
    -- o tonumber(...) aqui é obrigatório, senão a aritmética abaixo quebra
    currentTokens = tonumber(vals[1])
    lastTimestamp = tonumber(vals[2])
end

-- mesma proteção contra relógio andando pra trás que você já fez no
-- gerador de ID -- aqui, em vez de rejeitar com erro, simplesmente não
-- concede refill negativo (mais tolerante, já que o pior caso aqui é
-- "um pouco menos generoso", não uma colisão de dados)
local elapsed = now - lastTimestamp
if elapsed < 0 then
    elapsed = 0
end

local tokensToAdd = elapsed * refillRate
currentTokens = math.min(currentTokens + tokensToAdd, maxTokens)

local allowed = 0
if currentTokens >= requestCost then
    currentTokens = currentTokens - requestCost
    allowed = 1
end

redis.call('HMSET', KEYS[1], 'tokens', currentTokens, 'timestamp', now)

-- TTL: tempo que levaria pra esse bucket encher de novo do zero, mais uma
-- margem. Assim, buckets de chaves que pararam de ser usadas não ocupam
-- memória do Redis pra sempre.
local ttl = 3600
if refillRate > 0 then
    ttl = math.ceil(maxTokens / refillRate) + 60
end
redis.call('EXPIRE', KEYS[1], ttl)

return allowed