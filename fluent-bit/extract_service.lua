-- Extract service name from Docker container log path or fluentd tag.
-- Sets "service" and "level" fields for Loki labels.
function extract_service(tag, timestamp, record)
    local modified = false

    -- Extract container name from Docker log file path
    -- Path format: /var/lib/docker/containers/<id>/<id>-json.log
    -- The container_name field is set by the Docker JSON log driver
    if record["container_name"] then
        record["service"] = record["container_name"]:gsub("^/", "")
        modified = true
    elseif record["container_id"] then
        record["service"] = record["container_id"]:sub(1, 12)
        modified = true
    end

    -- Normalize level field from different log formats
    -- PostgreSQL jsonlog uses "error_severity"
    if record["error_severity"] then
        record["level"] = record["error_severity"]:lower()
        modified = true
    -- Go logrus uses "level" already
    elseif record["level"] then
        -- already set
    else
        record["level"] = "info"
        modified = true
    end

    if modified then
        return 1, timestamp, record
    end
    return 0, timestamp, record
end
