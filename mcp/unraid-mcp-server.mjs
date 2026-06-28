#!/usr/bin/env node

const helperUrl = (process.env.UNRAID_AI_HELPER_URL || "http://127.0.0.1:37231").replace(/\/+$/, "");
const apiKey = process.env.UNRAID_AI_API_KEY || "";

const tools = [
  {
    name: "unraid_health",
    description: "Check whether the Unraid AI helper is reachable.",
    inputSchema: {
      type: "object",
      properties: {},
      additionalProperties: false,
    },
  },
  {
    name: "unraid_inventory",
    description: "Read DockerMan XML inventory from the Unraid helper.",
    inputSchema: {
      type: "object",
      properties: {},
      additionalProperties: false,
    },
  },
  {
    name: "unraid_capabilities",
    description: "Read the helper capability map: implemented and planned safe action modules, providers and access levels.",
    inputSchema: {
      type: "object",
      properties: {},
      additionalProperties: false,
    },
  },
  {
    name: "unraid_docker_inspect",
    description: "Read normalized Docker inspect runtime inventory via the helper.",
    inputSchema: {
      type: "object",
      properties: {},
      additionalProperties: false,
    },
  },
  {
    name: "unraid_compare_runtime",
    description: "Compare DockerMan XML templates against Docker runtime state.",
    inputSchema: {
      type: "object",
      properties: {
        inspect_path: {
          type: "string",
          description: "Optional path on the helper host to a docker inspect JSON snapshot.",
        },
      },
      additionalProperties: false,
    },
  },
  {
    name: "unraid_discover_integrations",
    description: "Read-only discovery of known app integrations and appdata secret locations. Secret values are masked and not returned in full.",
    inputSchema: {
      type: "object",
      properties: {
        containers: {
          type: "array",
          items: { type: "string" },
          description: "Optional container names to limit discovery.",
        },
      },
      additionalProperties: false,
    },
  },
  {
    name: "unraid_plan_dashboard",
    description: "Create a read-only dashboard configuration plan through a provider adapter. Currently supports provider=amud.",
    inputSchema: {
      type: "object",
      properties: {
        provider: {
          type: "string",
          enum: ["amud"],
          description: "Dashboard provider adapter. Defaults to amud.",
        },
        local_host: { type: "string" },
        url_mode: { type: "string", enum: ["local", "cloudflare", "hybrid"] },
        cloudflare_domain: { type: "string" },
        cloudflare_routes: {
          type: "object",
          additionalProperties: { type: "string" },
        },
        containers: { type: "array", items: { type: "string" } },
        exclude_containers: { type: "array", items: { type: "string" } },
        include_port_only: {
          type: "boolean",
          description: "Also include templates without WebUI that only expose a TCP port. Defaults to false.",
        },
        runtime_filter: {
          type: "string",
          enum: ["templates", "existing", "running"],
          description: "Filter XML templates by Docker runtime state. Defaults to running when the helper has Docker access.",
        },
        inspect_path: {
          type: "string",
          description: "Optional path on the helper host to a docker inspect JSON snapshot for runtime filtering.",
        },
        include_diffs: { type: "boolean" },
        save_plan: { type: "boolean" },
      },
      additionalProperties: false,
    },
  },
  {
    name: "unraid_apply_dashboard",
    description: "Apply a previously reviewed dashboard plan. Requires exact confirm_plan_hash.",
    inputSchema: {
      type: "object",
      properties: {
        plan_path: { type: "string" },
        plan: { type: "object" },
        confirm_plan_hash: { type: "string" },
        approval_token: { type: "string" },
      },
      required: ["confirm_plan_hash"],
      additionalProperties: false,
    },
  },
  {
    name: "unraid_plan_dashboard_sync",
    description: "Create one read-only workflow plan for dashboard configuration, XML diff, optional DockerMan recreate, and runtime verification. Currently supports provider=amud.",
    inputSchema: {
      type: "object",
      properties: {
        provider: {
          type: "string",
          enum: ["amud"],
          description: "Dashboard provider adapter. Defaults to amud.",
        },
        local_host: { type: "string" },
        url_mode: { type: "string", enum: ["local", "cloudflare", "hybrid"] },
        cloudflare_domain: { type: "string" },
        cloudflare_routes: {
          type: "object",
          additionalProperties: { type: "string" },
        },
        containers: { type: "array", items: { type: "string" } },
        exclude_containers: { type: "array", items: { type: "string" } },
        include_port_only: {
          type: "boolean",
          description: "Also include templates without WebUI that only expose a TCP port. Defaults to false.",
        },
        runtime_filter: {
          type: "string",
          enum: ["templates", "existing", "running"],
          description: "Filter XML templates by Docker runtime state. Defaults to running when the helper has Docker access.",
        },
        inspect_path: {
          type: "string",
          description: "Optional path on the helper host to a docker inspect JSON snapshot for runtime filtering/recreate planning.",
        },
        recreate_mode: {
          type: "string",
          enum: ["changed", "all", "none"],
          description: "Which planned dashboard entries should be recreated after XML apply. Defaults to changed.",
        },
        include_diffs: { type: "boolean" },
        save_plan: { type: "boolean" },
      },
      additionalProperties: false,
    },
  },
  {
    name: "unraid_apply_dashboard_sync",
    description: "Apply a reviewed dashboard sync plan: XML changes, optional DockerMan recreate, and runtime verification. Requires exact confirm_plan_hash for the sync plan.",
    inputSchema: {
      type: "object",
      properties: {
        plan_path: { type: "string" },
        plan: { type: "object" },
        confirm_plan_hash: { type: "string" },
        approval_token: { type: "string" },
      },
      required: ["confirm_plan_hash"],
      additionalProperties: false,
    },
  },
  {
    name: "unraid_plan_amud",
    description: "Compatibility shortcut for AMUD Docker label planning. Prefer unraid_plan_dashboard for new workflows.",
    inputSchema: {
      type: "object",
      properties: {
        local_host: { type: "string" },
        url_mode: { type: "string", enum: ["local", "cloudflare", "hybrid"] },
        cloudflare_domain: { type: "string" },
        cloudflare_routes: {
          type: "object",
          additionalProperties: { type: "string" },
        },
        containers: { type: "array", items: { type: "string" } },
        exclude_containers: { type: "array", items: { type: "string" } },
        include_port_only: {
          type: "boolean",
          description: "Also include templates without WebUI that only expose a TCP port. Defaults to false.",
        },
        runtime_filter: {
          type: "string",
          enum: ["templates", "existing", "running"],
          description: "Filter XML templates by Docker runtime state. Defaults to running when the helper has Docker access.",
        },
        inspect_path: {
          type: "string",
          description: "Optional path on the helper host to a docker inspect JSON snapshot for runtime filtering.",
        },
        include_diffs: { type: "boolean" },
        save_plan: { type: "boolean" },
      },
      additionalProperties: false,
    },
  },
  {
    name: "unraid_apply_amud",
    description: "Compatibility shortcut for applying AMUD Docker label plans. Prefer unraid_apply_dashboard for new workflows.",
    inputSchema: {
      type: "object",
      properties: {
        plan_path: { type: "string" },
        plan: { type: "object" },
        confirm_plan_hash: { type: "string" },
        approval_token: { type: "string" },
      },
      required: ["confirm_plan_hash"],
      additionalProperties: false,
    },
  },
  {
    name: "unraid_plan_tz",
    description: "Create a read-only plan to set DockerMan TZ variable, usually Europe/Prague.",
    inputSchema: {
      type: "object",
      properties: {
        timezone: { type: "string" },
        containers: { type: "array", items: { type: "string" } },
        include_unchanged: { type: "boolean" },
        include_diffs: { type: "boolean" },
        save_plan: { type: "boolean" },
      },
      additionalProperties: false,
    },
  },
  {
    name: "unraid_apply_tz",
    description: "Apply a previously reviewed TZ plan. Requires exact confirm_plan_hash.",
    inputSchema: {
      type: "object",
      properties: {
        plan_path: { type: "string" },
        plan: { type: "object" },
        confirm_plan_hash: { type: "string" },
        approval_token: { type: "string" },
      },
      required: ["confirm_plan_hash"],
      additionalProperties: false,
    },
  },
  {
    name: "unraid_plan_recreate",
    description: "Create a read-only recreate plan for containers whose runtime state differs from XML.",
    inputSchema: {
      type: "object",
      properties: {
        inspect_path: { type: "string" },
        containers: { type: "array", items: { type: "string" } },
        all: { type: "boolean" },
        save_plan: { type: "boolean" },
      },
      additionalProperties: false,
    },
  },
  {
    name: "unraid_apply_recreate",
    description: "Apply a previously reviewed recreate plan using Unraid DockerMan rebuild_container. Requires exact confirm_plan_hash.",
    inputSchema: {
      type: "object",
      properties: {
        plan_path: { type: "string" },
        plan: { type: "object" },
        confirm_plan_hash: { type: "string" },
        approval_token: { type: "string" },
      },
      required: ["confirm_plan_hash"],
      additionalProperties: false,
    },
  },
  {
    name: "unraid_restore_xml",
    description: "Restore a DockerMan XML template from backup. Requires exact backup SHA256.",
    inputSchema: {
      type: "object",
      properties: {
        backup_path: { type: "string" },
        target_path: { type: "string" },
        confirm_backup_sha256: { type: "string" },
      },
      required: ["backup_path", "target_path", "confirm_backup_sha256"],
      additionalProperties: false,
    },
  },
];

const toolHandlers = {
  unraid_health: () => helperGet("/v1/health"),
  unraid_capabilities: () => helperGet("/v1/capabilities"),
  unraid_inventory: () => helperGet("/v1/inventory"),
  unraid_docker_inspect: () => helperGet("/v1/docker/inspect"),
  unraid_compare_runtime: (args) => {
    const query = args?.inspect_path ? `?inspect_path=${encodeURIComponent(args.inspect_path)}` : "";
    return helperGet(`/v1/runtime/compare${query}`);
  },
  unraid_discover_integrations: (args) => helperPost("/v1/discover/integrations", args || {}),
  unraid_plan_dashboard: (args) => helperPost("/v1/plan/dashboard", args || {}),
  unraid_apply_dashboard: (args) => helperPost("/v1/apply/dashboard", args || {}),
  unraid_plan_dashboard_sync: (args) => helperPost("/v1/plan/dashboard-sync", args || {}),
  unraid_apply_dashboard_sync: (args) => helperPost("/v1/apply/dashboard-sync", args || {}),
  unraid_plan_amud: (args) => helperPost("/v1/plan/amud", args || {}),
  unraid_apply_amud: (args) => helperPost("/v1/apply/amud", args || {}),
  unraid_plan_tz: (args) => helperPost("/v1/plan/tz", args || {}),
  unraid_apply_tz: (args) => helperPost("/v1/apply/tz", args || {}),
  unraid_plan_recreate: (args) => helperPost("/v1/plan/recreate", args || {}),
  unraid_apply_recreate: (args) => helperPost("/v1/apply/recreate", args || {}),
  unraid_restore_xml: (args) => helperPost("/v1/restore/xml", args || {}),
};

let inputBuffer = "";
process.stdin.setEncoding("utf8");
process.stdin.on("data", (chunk) => {
  inputBuffer += chunk;
  while (true) {
    const newline = inputBuffer.indexOf("\n");
    if (newline === -1) {
      break;
    }
    const line = inputBuffer.slice(0, newline).trim();
    inputBuffer = inputBuffer.slice(newline + 1);
    if (line) {
      void handleLine(line);
    }
  }
});

process.stdin.on("end", () => {
  const line = inputBuffer.trim();
  inputBuffer = "";
  if (line) {
    void handleLine(line);
    return;
  }
});

async function handleLine(line) {
  let message;
  try {
    message = JSON.parse(line);
  } catch (error) {
    writeResponse(null, null, rpcError(-32700, `Parse error: ${error.message}`));
    return;
  }

  if (Array.isArray(message)) {
    for (const item of message) {
      await handleMessage(item);
    }
    return;
  }
  await handleMessage(message);
}

async function handleMessage(message) {
  if (!message || typeof message !== "object") {
    writeResponse(null, null, rpcError(-32600, "Invalid Request"));
    return;
  }
  const hasId = Object.prototype.hasOwnProperty.call(message, "id");
  try {
    const result = await dispatch(message.method, message.params || {});
    if (hasId) {
      writeResponse(message.id, result, null);
    }
  } catch (error) {
    if (hasId) {
      writeResponse(message.id, null, rpcError(error.code || -32000, error.message || String(error)));
    }
  }
}

async function dispatch(method, params) {
  switch (method) {
    case "initialize":
      return {
        protocolVersion: params.protocolVersion || "2025-06-18",
        capabilities: {
          tools: {},
        },
        serverInfo: {
          name: "unraid-ai-manager",
          version: "0.1.7",
        },
      };
    case "tools/list":
      return { tools };
    case "tools/call":
      return callTool(params);
    case "resources/list":
      return { resources: [] };
    case "prompts/list":
      return { prompts: [] };
    case "ping":
      return {};
    default:
      throw Object.assign(new Error(`Method not found: ${method}`), { code: -32601 });
  }
}

async function callTool(params) {
  const name = params?.name;
  const args = params?.arguments || {};
  const handler = toolHandlers[name];
  if (!handler) {
    throw Object.assign(new Error(`Unknown tool: ${name}`), { code: -32602 });
  }
  const result = await handler(args);
  return {
    content: [
      {
        type: "text",
        text: JSON.stringify(result, null, 2),
      },
    ],
    isError: false,
  };
}

async function helperGet(path) {
  return helperRequest("GET", path, undefined);
}

async function helperPost(path, body) {
  return helperRequest("POST", path, body);
}

async function helperRequest(method, path, body) {
  const headers = {
    Accept: "application/json",
  };
  let requestBody;
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
    requestBody = JSON.stringify(body);
  }
  if (apiKey) {
    headers["X-Unraid-AI-Key"] = apiKey;
  }

  const response = await fetch(`${helperUrl}${path}`, {
    method,
    headers,
    body: requestBody,
  });
  const text = await response.text();
  let payload;
  try {
    payload = text ? JSON.parse(text) : {};
  } catch {
    payload = { raw: text };
  }
  if (!response.ok) {
    const message = payload?.error || `Helper HTTP ${response.status}`;
    throw Object.assign(new Error(message), { code: -32000, data: payload });
  }
  return payload;
}

function writeResponse(id, result, error) {
  const response = {
    jsonrpc: "2.0",
    id,
  };
  if (error) {
    response.error = error;
  } else {
    response.result = result;
  }
  process.stdout.write(`${JSON.stringify(response)}\n`);
}

function rpcError(code, message, data) {
  const error = { code, message };
  if (data !== undefined) {
    error.data = data;
  }
  return error;
}
