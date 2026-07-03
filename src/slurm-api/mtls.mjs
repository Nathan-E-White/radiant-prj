const https = require('https');
const fs = require('fs');

// Load the cryptographic assets safely from disk storage
const options = {
    hostname: 'localhost',
    port: 8443,
    path: '/api/jobs/submit',
    method: 'POST',
    headers: {
        'Content-Type': 'application/json',
    },
    // --- mTLS Configuration ---
    key: fs.readFileSync('client.key'),  // Proves our identity to Go
    cert: fs.readFileSync('client.crt'), // Proves our identity to Go
    ca: fs.readFileSync('ca.crt'),       // Verifies Go's identity to us
    rejectUnauthorized: true             // Drop connection if Go fails verification
};

// Send the request
const req = https.request(options, (res) => {
    let data = '';
    res.on('data', (chunk) => data += chunk);
    res.on('end', () => console.log('Response from Go Cluster API:', data));
});

req.on('error', (e) => console.error('Cryptographic handshake failed:', e));

req.write(JSON.stringify({
    node_count: 2,
    partition: 'gpu',
    script_name: 'matrix-multiplication'
}));
req.end();
