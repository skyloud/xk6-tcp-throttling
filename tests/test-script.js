import http from 'k6/http';
import tcpThrottle from 'k6/x/tcp-throttling';
import { check } from 'k6';

export let options = {
  stages: [
    { duration: '30s', target: 1 },
    { duration: '60s', target: 3 },
    { duration: '30s', target: 0 },
  ],
};

let totalBytesReceived = 0;
let startTime = Date.now();

export default function() {
  // Test HTTP sans throttling
  let response = http.get(`${__ENV.SERVER_URL}/test?size=1048576`);
  check(response, {
    'status is 200': (r) => r.status === 200,
  });
  
  // Test TCP avec throttling
  try {
    const conn = tcpThrottle.connect('server:8080');
    
    // Limite à 500KB/s
    conn.setBandwidthLimit(500 * 1024);
    
    // Envoie requête HTTP basique
    conn.write('GET /test?size=1048576 HTTP/1.1\r\nHost: server\r\n\r\n');
    
    let totalReceived = 0;
    let startRead = Date.now();
    
    // Lit la réponse avec throttling
    while (totalReceived < 1048576) {
      let data = conn.readWithThrottle(8192);
      totalReceived += data.length;
      totalBytesReceived += data.length;
    }
    
    let duration = (Date.now() - startRead) / 1000;
    let throughput = (totalReceived / duration) / 1024 / 1024;
    
    console.log(`TCP Throttled: ${totalReceived} bytes in ${duration}s (${throughput.toFixed(2)} MB/s)`);
    
    conn.close();
  } catch (e) {
    console.error('TCP test failed:', e);
  }
}

export function handleSummary(data) {
  let elapsed = (Date.now() - startTime) / 1000;
  let avgThroughput = (totalBytesReceived / elapsed) / 1024 / 1024;
  
  console.log(`\n=== CLIENT METRICS ===`);
  console.log(`Total bytes received: ${totalBytesReceived}`);
  console.log(`Test duration: ${elapsed.toFixed(2)}s`);
  console.log(`Average throughput: ${avgThroughput.toFixed(2)} MB/s`);
  
  return {
    '/results/summary.json': JSON.stringify({
      totalBytesReceived,
      testDuration: elapsed,
      avgThroughput,
      ...data
    }, null, 2)
  };
}