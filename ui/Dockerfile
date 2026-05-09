from node:24 as builder
workdir /app
copy package*.json ./
run npm ci
copy . .
run NODE_OPTIONS="--max-old-space-size=4096" npm run build

from node:20-alpine
env NODE_ENV=production
workdir /app
copy package*.json ./
run npm ci --omit=dev
copy --from=builder /app/build ./build
user node
expose 3000
cmd ["node", "build"]
