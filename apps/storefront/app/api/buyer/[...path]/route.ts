import { handleBuyerRequest } from "../../../../lib/buyer-server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export const GET = handleBuyerRequest;
export const POST = handleBuyerRequest;
export const PUT = handleBuyerRequest;
export const PATCH = handleBuyerRequest;
export const DELETE = handleBuyerRequest;
export const HEAD = handleBuyerRequest;
export const OPTIONS = handleBuyerRequest;
