import tcpThrottle from 'k6/x/tcp-throttling';
import { scenario } from 'k6/execution';
import http from 'k6/http';
import { Trend, Counter } from 'k6/metrics';

export let options = {
  scenarios: {
    stage1: {
      executor: 'shared-iterations',
      vus: 1,
      iterations: 1,
      startTime: '0s',
      env: { BANDWIDTH: '512000', STAGE: 'Stage 1: 500 KB/s', STAGE_ID: '1' },
    },
    stage2: {
      executor: 'shared-iterations',
      vus: 1,
      iterations: 1,
      startTime: '65s',
      env: { BANDWIDTH: '1048576', STAGE: 'Stage 2: 1 MB/s', STAGE_ID: '2' },
    },
    stage3: {
      executor: 'shared-iterations',
      vus: 1,
      iterations: 1,
      startTime: '100s',
      env: { BANDWIDTH: '10485760', STAGE: 'Stage 3: 10 MB/s', STAGE_ID: '3' },
    },
  },
};

const PAYLOAD_SIZE = 30 * 1024 * 1024; // 30 MB
const throughputMetric = new Trend('download_throughput_mbps', true);
const durationMetric = new Trend('download_duration_s', true);
const payloadMetric = new Counter('download_payload_mb');

export default function() {
  const bandwidth = parseInt(__ENV.BANDWIDTH);
  const stageName = __ENV.STAGE;
  
  try {
    const conn = tcpThrottle.connect('server:8080');
    
    conn.setReceiveBuffer(4096);
    conn.setBandwidthLimit(bandwidth);
    
    conn.write(`GET /test?size=${PAYLOAD_SIZE} HTTP/1.1\r\nHost: server\r\nConnection: close\r\n\r\n`);
    
    let totalReceived = 0;
    let payloadReceived = 0;
    let startRead = Date.now();
    let inBody = false;
    
    // Lit toute la réponse avec throttling (headers + body)
    while (true) {
      let data = conn.readWithThrottle(8192);
      if (data.length === 0) break;
      
      totalReceived += data.length;
      
      // Compter seulement le payload après les headers
      if (!inBody) {
        let dataStr = String.fromCharCode.apply(null, data);
        let bodyStart = dataStr.indexOf('\r\n\r\n');
        if (bodyStart !== -1) {
          inBody = true;
          payloadReceived += data.length - (bodyStart + 4);
        }
      } else {
        payloadReceived += data.length;
      }
      

    }
    
    let duration = (Date.now() - startRead) / 1000;
    let throughput = (payloadReceived / duration) / 1024 / 1024;
    let payloadMB = payloadReceived / 1024 / 1024;
    
    // Enregistrer les métriques avec tags
    const tags = { 
      stage: stageName,
      stage_id: __ENV.STAGE_ID,
      bandwidth_limit: (bandwidth / 1024 / 1024).toFixed(2) + ' MB/s'
    };
    
    throughputMetric.add(throughput, tags);
    durationMetric.add(duration, tags);
    payloadMetric.add(payloadMB, tags);
    
    console.log(`${stageName} - Downloaded ${(payloadReceived / 1024 / 1024).toFixed(2)} MB in ${duration.toFixed(2)}s (${throughput.toFixed(2)} MB/s)`);
    
    conn.close();
  } catch (e) {
    console.error('TCP test failed:', e);
  }
}

export function handleSummary(data) {
  // Extraire les résultats client depuis les métriques
  const clientResults = [];
  
  // K6 stocke les métriques avec tags dans thresholds.values
  const throughputData = data.metrics.download_throughput_mbps;
  const durationData = data.metrics.download_duration_s;
  const payloadData = data.metrics.download_payload_mb;
  
  // Collecter tous les tags uniques
  const stageMap = {};
  
  if (throughputData && throughputData.values) {
    for (const key in throughputData.values) {
      const value = throughputData.values[key];
      if (value && value.avg !== undefined) {
        const stageId = key.split(':')[0] || key;
        if (!stageMap[stageId]) {
          stageMap[stageId] = {};
        }
        stageMap[stageId].throughput = value.avg;
      }
    }
  }
  
  if (durationData && durationData.values) {
    for (const key in durationData.values) {
      const value = durationData.values[key];
      if (value && value.avg !== undefined) {
        const stageId = key.split(':')[0] || key;
        if (!stageMap[stageId]) {
          stageMap[stageId] = {};
        }
        stageMap[stageId].duration = value.avg;
      }
    }
  }
  
  if (payloadData && payloadData.values) {
    for (const key in payloadData.values) {
      const value = payloadData.values[key];
      if (value && value.count !== undefined) {
        const stageId = key.split(':')[0] || key;
        if (!stageMap[stageId]) {
          stageMap[stageId] = {};
        }
        stageMap[stageId].payload = value.count;
      }
    }
  }
  
  // Construire les résultats
  for (const [stageId, values] of Object.entries(stageMap)) {
    clientResults.push({
      stage: `Stage ${stageId}`,
      throughput_mbps: values.throughput?.toFixed(2) || 'N/A',
      duration_s: values.duration?.toFixed(2) || 'N/A',
      payload_mb: values.payload?.toFixed(2) || 'N/A'
    });
  }
  
  // Si aucun résultat via métriques, utiliser les données brutes du summary
  if (clientResults.length === 0) {
    console.log('DEBUG: Full data structure:', JSON.stringify(data, null, 2));
  }
  
  // Déclencher la sauvegarde des résultats serveur
  try {
    http.get('http://server:8080/save-results');
  } catch (e) {
    console.error('Failed to save server results:', e);
  }
  
  return {
    'stdout': '',
    '/results/client-results.json': JSON.stringify(clientResults, null, 2),
    '/results/k6-summary.json': JSON.stringify(data, null, 2)
  };
}