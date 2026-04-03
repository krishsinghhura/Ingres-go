from fastapi import FastAPI
from routers import dev

app = FastAPI(title="Ingres Analytics API", version="1.0.0")

app.include_router(dev.router)

@app.get("/health")
def health_check():
    return {"status": "ok", "service": "analytics-microservice"}
