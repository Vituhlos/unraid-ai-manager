<?php
$plugin = "unraid-ai-manager";
$config = "/boot/config/plugins/$plugin/unraid-ai-manager.cfg";
$rc = "/etc/rc.d/rc.unraid-ai-manager";
$message = "";
$error = "";

function uaim_h($value) {
  return htmlspecialchars($value ?? "", ENT_QUOTES | ENT_SUBSTITUTE, "UTF-8");
}

function uaim_cfg_read($path) {
  $result = [];
  if (!is_file($path)) {
    return $result;
  }
  foreach (file($path, FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES) as $line) {
    if ($line === "" || $line[0] === "#" || strpos($line, "=") === false) {
      continue;
    }
    [$key, $value] = explode("=", $line, 2);
    $result[$key] = trim($value, "\"'");
  }
  return $result;
}

function uaim_cfg_default($cfg, $key, $fallback) {
  return array_key_exists($key, $cfg) && $cfg[$key] !== "" ? $cfg[$key] : $fallback;
}

function uaim_shell_quote_cfg($value) {
  return '"' . str_replace(['\\', '"', '$', '`'], ['\\\\', '\\"', '\\$', '\\`'], $value) . '"';
}

function uaim_write_config($path, $cfg) {
  $dir = dirname($path);
  if (!is_dir($dir)) {
    mkdir($dir, 0700, true);
  }
  $lines = [];
  foreach ($cfg as $key => $value) {
    $lines[] = $key . "=" . uaim_shell_quote_cfg($value);
  }
  file_put_contents($path, implode("\n", $lines) . "\n", LOCK_EX);
  chmod($path, 0600);
}

function uaim_generate_key() {
  return bin2hex(random_bytes(32));
}

$cfg = uaim_cfg_read($config);
$defaults = [
  "LISTEN" => "127.0.0.1:37231",
  "TEMPLATES_DIR" => "/boot/config/plugins/dockerMan/templates-user",
  "BACKUP_DIR" => "/mnt/user/appdata/unraid-ai-manager/backups",
  "AUDIT_DIR" => "/mnt/user/appdata/unraid-ai-manager/audit",
  "PLANS_DIR" => "/mnt/user/appdata/unraid-ai-manager/plans",
  "APPROVALS_DIR" => "/mnt/user/appdata/unraid-ai-manager/approvals",
  "DOCKER_SOCKET" => "/var/run/docker.sock",
  "DOCKER_HOST" => "",
  "LOCAL_HOST" => "",
  "API_KEY" => "",
  "REQUIRE_APPROVAL_TOKEN" => "true",
];

foreach ($defaults as $key => $value) {
  $cfg[$key] = uaim_cfg_default($cfg, $key, $value);
}

if ($_SERVER["REQUEST_METHOD"] === "POST") {
  $action = $_POST["action"] ?? "";
  try {
    if ($action === "save" || $action === "save_restart") {
      $cfg["LISTEN"] = trim($_POST["LISTEN"] ?? $cfg["LISTEN"]);
      $cfg["TEMPLATES_DIR"] = trim($_POST["TEMPLATES_DIR"] ?? $cfg["TEMPLATES_DIR"]);
      $cfg["BACKUP_DIR"] = trim($_POST["BACKUP_DIR"] ?? $cfg["BACKUP_DIR"]);
      $cfg["AUDIT_DIR"] = trim($_POST["AUDIT_DIR"] ?? $cfg["AUDIT_DIR"]);
      $cfg["PLANS_DIR"] = trim($_POST["PLANS_DIR"] ?? $cfg["PLANS_DIR"]);
      $cfg["APPROVALS_DIR"] = trim($_POST["APPROVALS_DIR"] ?? $cfg["APPROVALS_DIR"]);
      $cfg["DOCKER_SOCKET"] = trim($_POST["DOCKER_SOCKET"] ?? $cfg["DOCKER_SOCKET"]);
      $cfg["DOCKER_HOST"] = trim($_POST["DOCKER_HOST"] ?? $cfg["DOCKER_HOST"]);
      $cfg["LOCAL_HOST"] = trim($_POST["LOCAL_HOST"] ?? $cfg["LOCAL_HOST"]);
      $cfg["REQUIRE_APPROVAL_TOKEN"] = isset($_POST["REQUIRE_APPROVAL_TOKEN"]) ? "true" : "false";
      if (($cfg["API_KEY"] ?? "") === "") {
        $cfg["API_KEY"] = uaim_generate_key();
      }
      uaim_write_config($config, $cfg);
      $message = "Configuration saved.";
      if ($action === "save_restart") {
        $output = shell_exec("$rc restart 2>&1");
        $message .= "\n" . trim($output ?? "");
      }
    } elseif ($action === "generate_key") {
      $cfg["API_KEY"] = uaim_generate_key();
      uaim_write_config($config, $cfg);
      $message = "New API key generated. Restart helper for active sessions to use it.";
    } elseif (in_array($action, ["start", "stop", "restart", "status"], true)) {
      $output = shell_exec("$rc " . escapeshellarg($action) . " 2>&1");
      $message = trim($output ?? "");
    }
  } catch (Throwable $e) {
    $error = $e->getMessage();
  }
}

$statusOutput = shell_exec("$rc status 2>&1");
$status = trim($statusOutput ?? "");
$hasKey = ($cfg["API_KEY"] ?? "") !== "";
$keyPreview = $hasKey ? substr($cfg["API_KEY"], 0, 8) . "..." . substr($cfg["API_KEY"], -8) : "not configured";
?>

<div class="unraid-ai-manager">
  <h2>Unraid AI Manager</h2>

  <p>
    This plugin runs the local Unraid helper used by the PC-side MCP server.
    The helper exposes only named API actions; it does not accept raw shell commands.
  </p>

  <?php if ($message): ?>
    <blockquote style="white-space: pre-wrap;"><?= uaim_h($message) ?></blockquote>
  <?php endif; ?>
  <?php if ($error): ?>
    <blockquote style="white-space: pre-wrap; color: #f66;"><?= uaim_h($error) ?></blockquote>
  <?php endif; ?>

  <h3>Status</h3>
  <pre><?= uaim_h($status) ?></pre>

  <form method="post">
    <button type="submit" name="action" value="start">Start</button>
    <button type="submit" name="action" value="stop">Stop</button>
    <button type="submit" name="action" value="restart">Restart</button>
    <button type="submit" name="action" value="status">Refresh status</button>
  </form>

  <h3>Configuration</h3>
  <form method="post">
    <table class="share_status">
      <tr><td>Listen</td><td><input type="text" name="LISTEN" value="<?= uaim_h($cfg["LISTEN"]) ?>" style="width: 420px;"></td></tr>
      <tr><td>Templates</td><td><input type="text" name="TEMPLATES_DIR" value="<?= uaim_h($cfg["TEMPLATES_DIR"]) ?>" style="width: 650px;"></td></tr>
      <tr><td>Backup dir</td><td><input type="text" name="BACKUP_DIR" value="<?= uaim_h($cfg["BACKUP_DIR"]) ?>" style="width: 650px;"></td></tr>
      <tr><td>Audit dir</td><td><input type="text" name="AUDIT_DIR" value="<?= uaim_h($cfg["AUDIT_DIR"]) ?>" style="width: 650px;"></td></tr>
      <tr><td>Plans dir</td><td><input type="text" name="PLANS_DIR" value="<?= uaim_h($cfg["PLANS_DIR"]) ?>" style="width: 650px;"></td></tr>
      <tr><td>Approvals dir</td><td><input type="text" name="APPROVALS_DIR" value="<?= uaim_h($cfg["APPROVALS_DIR"]) ?>" style="width: 650px;"></td></tr>
      <tr><td>Docker socket</td><td><input type="text" name="DOCKER_SOCKET" value="<?= uaim_h($cfg["DOCKER_SOCKET"]) ?>" style="width: 650px;"></td></tr>
      <tr><td>Docker host</td><td><input type="text" name="DOCKER_HOST" value="<?= uaim_h($cfg["DOCKER_HOST"]) ?>" style="width: 650px;"></td></tr>
      <tr><td>Default local host/IP</td><td><input type="text" name="LOCAL_HOST" value="<?= uaim_h($cfg["LOCAL_HOST"]) ?>" style="width: 420px;" placeholder="192.0.2.10"></td></tr>
      <tr><td>API key</td><td><code><?= uaim_h($keyPreview) ?></code> <button type="submit" name="action" value="generate_key">Generate new key</button></td></tr>
      <tr><td>Require approval token</td><td><input type="checkbox" name="REQUIRE_APPROVAL_TOKEN" value="true" <?= ($cfg["REQUIRE_APPROVAL_TOKEN"] === "true") ? "checked" : "" ?>> recommended</td></tr>
    </table>

    <p>
      <button type="submit" name="action" value="save">Save</button>
      <button type="submit" name="action" value="save_restart">Save & Restart</button>
    </p>
  </form>

  <h3>Useful commands</h3>
  <pre>
/etc/rc.d/rc.unraid-ai-manager status
/etc/rc.d/rc.unraid-ai-manager restart

/usr/local/bin/unraid-ai-manager discover-integrations \
  --templates <?= uaim_h($cfg["TEMPLATES_DIR"]) ?>

/usr/local/bin/unraid-ai-manager plan-dashboard-sync \
  --provider amud \
  --templates <?= uaim_h($cfg["TEMPLATES_DIR"]) ?> \
  --local-host <?= uaim_h($cfg["LOCAL_HOST"] ?: "192.0.2.10") ?> \
  --url-mode local \
  --runtime-filter running \
  --recreate-mode changed \
  --docker-socket <?= uaim_h($cfg["DOCKER_SOCKET"]) ?> \
  --diff \
  --out <?= uaim_h($cfg["PLANS_DIR"]) ?>/dashboard-sync-plan.json

/usr/local/bin/unraid-ai-manager approve-plan \
  --plan <?= uaim_h($cfg["PLANS_DIR"]) ?>/PLAN.json \
  --approvals-dir <?= uaim_h($cfg["APPROVALS_DIR"]) ?> \
  --purpose dashboard-sync \
  --ttl 15m

/usr/local/bin/unraid-ai-manager apply-dashboard-sync-plan \
  --plan <?= uaim_h($cfg["PLANS_DIR"]) ?>/dashboard-sync-plan.json \
  --confirm-plan-hash HASH \
  --backup-dir <?= uaim_h($cfg["BACKUP_DIR"]) ?> \
  --audit-dir <?= uaim_h($cfg["AUDIT_DIR"]) ?> \
  --docker-socket <?= uaim_h($cfg["DOCKER_SOCKET"]) ?> \
  </pre>

  <h3>PC connection</h3>
  <p>Recommended: keep the helper bound to <code>127.0.0.1</code> and connect from your PC using an SSH tunnel:</p>
  <pre>ssh -L 37231:127.0.0.1:37231 root@<?= uaim_h($_SERVER['SERVER_NAME'] ?? 'unraid') ?></pre>
  <p>
    MCP server environment on PC:
  </p>
  <pre>
UNRAID_AI_HELPER_URL=http://127.0.0.1:37231
UNRAID_AI_API_KEY=&lt;the API key from <?= uaim_h($config) ?>&gt;
  </pre>
</div>
