#!/bin/sh
# Nginx security setup script for Alchemorsel v3

set -e

echo "Setting up nginx security configuration..."

# Create necessary directories
mkdir -p /var/log/nginx /var/cache/nginx

# Set proper permissions
chown -R nginx:nginx /var/log/nginx /var/cache/nginx /usr/share/nginx/html

# Generate self-signed SSL certificate if not provided
if [ ! -f /etc/ssl/certs/alchemorsel.crt ]; then
    echo "Generating self-signed SSL certificate..."
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
        -keyout /etc/ssl/private/alchemorsel.key \
        -out /etc/ssl/certs/alchemorsel.crt \
        -subj "/C=US/ST=CA/L=San Francisco/O=Alchemorsel/CN=alchemorsel.com"
    
    chmod 600 /etc/ssl/private/alchemorsel.key
    chmod 644 /etc/ssl/certs/alchemorsel.crt
fi

# Create basic auth file if not exists (for staging)
if [ ! -f /etc/nginx/.htpasswd ]; then
    echo "Creating basic auth file for staging..."
    # Default: admin/staging123 (should be changed in production)
    echo 'admin:$apr1$ruca84Hq$mbjdMZBAG.KWn7vfN/SNK/' > /etc/nginx/.htpasswd
fi

# Test nginx configuration
echo "Testing nginx configuration..."
nginx -t

echo "Nginx security setup completed."