#!/usr/bin/env node

// Performance Regression Detection Script for Alchemorsel v3
// Compares current performance metrics with baseline

const fs = require('fs');
const path = require('path');

const DEFAULT_THRESHOLDS = {
  responseTime: {
    p50: { threshold: 10, unit: 'percent' },   // 10% increase in median response time
    p90: { threshold: 15, unit: 'percent' },   // 15% increase in 90th percentile
    p95: { threshold: 20, unit: 'percent' },   // 20% increase in 95th percentile
    p99: { threshold: 25, unit: 'percent' },   // 25% increase in 99th percentile
  },
  errorRate: {
    threshold: 0.5,  // 0.5% absolute increase in error rate
    unit: 'percent'
  },
  throughput: {
    threshold: -10,  // 10% decrease in throughput
    unit: 'percent'
  }
};

class PerformanceComparator {
  constructor(options = {}) {
    this.currentFile = options.current || 'test/performance/results.json';
    this.baselineFile = options.baseline || 'test/performance/baseline.json';
    this.threshold = options.threshold || 10;
    this.outputFile = options.output || 'test/performance/comparison-report.json';
    this.thresholds = { ...DEFAULT_THRESHOLDS, ...options.customThresholds };
  }

  async compare() {
    try {
      console.log('🔍 Starting performance comparison...');
      
      const currentMetrics = this.loadMetrics(this.currentFile);
      const baselineMetrics = this.loadMetrics(this.baselineFile);
      
      if (!currentMetrics || !baselineMetrics) {
        throw new Error('Failed to load metrics files');
      }
      
      const comparison = this.performComparison(currentMetrics, baselineMetrics);
      const regressions = this.detectRegressions(comparison);
      
      this.generateReport(comparison, regressions);
      
      if (regressions.length > 0) {
        console.error('❌ Performance regressions detected!');
        process.exit(1);
      } else {
        console.log('✅ No performance regressions detected');
        process.exit(0);
      }
    } catch (error) {
      console.error('Error during performance comparison:', error.message);
      process.exit(1);
    }
  }

  loadMetrics(filePath) {
    try {
      console.log(`📊 Loading metrics from ${filePath}`);
      
      if (!fs.existsSync(filePath)) {
        console.warn(`Warning: Metrics file ${filePath} does not exist`);
        return null;
      }
      
      const content = fs.readFileSync(filePath, 'utf8');
      const metrics = JSON.parse(content);
      
      console.log(`✅ Loaded metrics from ${filePath}`);
      return metrics;
    } catch (error) {
      console.error(`Error loading metrics from ${filePath}:`, error.message);
      return null;
    }
  }

  performComparison(current, baseline) {
    console.log('🔄 Performing performance comparison...');
    
    const comparison = {
      timestamp: new Date().toISOString(),
      baseline: {
        timestamp: baseline.timestamp || 'unknown',
        summary: baseline.summary || {}
      },
      current: {
        timestamp: current.timestamp || new Date().toISOString(),
        summary: current.summary || {}
      },
      comparisons: {}
    };

    // Compare response times
    if (current.summary.responseTime && baseline.summary.responseTime) {
      comparison.comparisons.responseTime = this.compareResponseTimes(
        current.summary.responseTime,
        baseline.summary.responseTime
      );
    }

    // Compare error rates
    if (current.summary.errorRate !== undefined && baseline.summary.errorRate !== undefined) {
      comparison.comparisons.errorRate = this.compareErrorRates(
        current.summary.errorRate,
        baseline.summary.errorRate
      );
    }

    // Compare throughput
    if (current.summary.throughput && baseline.summary.throughput) {
      comparison.comparisons.throughput = this.compareThroughput(
        current.summary.throughput,
        baseline.summary.throughput
      );
    }

    // Compare by endpoint
    if (current.endpoints && baseline.endpoints) {
      comparison.comparisons.endpoints = this.compareEndpoints(
        current.endpoints,
        baseline.endpoints
      );
    }

    return comparison;
  }

  compareResponseTimes(current, baseline) {
    const percentiles = ['p50', 'p90', 'p95', 'p99'];
    const comparison = {};

    percentiles.forEach(percentile => {
      if (current[percentile] && baseline[percentile]) {
        const currentValue = current[percentile];
        const baselineValue = baseline[percentile];
        const change = ((currentValue - baselineValue) / baselineValue) * 100;
        
        comparison[percentile] = {
          current: currentValue,
          baseline: baselineValue,
          change: change,
          changeType: change > 0 ? 'increase' : 'decrease',
          significant: Math.abs(change) > this.thresholds.responseTime[percentile].threshold
        };
      }
    });

    return comparison;
  }

  compareErrorRates(current, baseline) {
    const change = current - baseline;
    
    return {
      current: current,
      baseline: baseline,
      change: change,
      changeType: change > 0 ? 'increase' : 'decrease',
      significant: Math.abs(change) > this.thresholds.errorRate.threshold
    };
  }

  compareThroughput(current, baseline) {
    const change = ((current - baseline) / baseline) * 100;
    
    return {
      current: current,
      baseline: baseline,
      change: change,
      changeType: change > 0 ? 'increase' : 'decrease',
      significant: change < this.thresholds.throughput.threshold
    };
  }

  compareEndpoints(current, baseline) {
    const endpointComparisons = {};
    
    // Compare common endpoints
    const commonEndpoints = Object.keys(current).filter(endpoint => 
      baseline.hasOwnProperty(endpoint)
    );
    
    commonEndpoints.forEach(endpoint => {
      endpointComparisons[endpoint] = {
        responseTime: this.compareResponseTimes(
          current[endpoint].responseTime || {},
          baseline[endpoint].responseTime || {}
        ),
        errorRate: current[endpoint].errorRate !== undefined && 
                  baseline[endpoint].errorRate !== undefined ? 
          this.compareErrorRates(
            current[endpoint].errorRate,
            baseline[endpoint].errorRate
          ) : null
      };
    });

    return endpointComparisons;
  }

  detectRegressions(comparison) {
    console.log('🔍 Detecting performance regressions...');
    
    const regressions = [];

    // Check response time regressions
    if (comparison.comparisons.responseTime) {
      Object.entries(comparison.comparisons.responseTime).forEach(([percentile, data]) => {
        if (data.significant && data.changeType === 'increase') {
          regressions.push({
            type: 'responseTime',
            metric: percentile,
            severity: this.calculateSeverity(data.change, this.thresholds.responseTime[percentile].threshold),
            current: data.current,
            baseline: data.baseline,
            change: data.change,
            message: `Response time ${percentile} increased by ${data.change.toFixed(2)}%`
          });
        }
      });
    }

    // Check error rate regressions
    if (comparison.comparisons.errorRate && comparison.comparisons.errorRate.significant) {
      if (comparison.comparisons.errorRate.changeType === 'increase') {
        regressions.push({
          type: 'errorRate',
          metric: 'overall',
          severity: this.calculateSeverity(
            comparison.comparisons.errorRate.change,
            this.thresholds.errorRate.threshold
          ),
          current: comparison.comparisons.errorRate.current,
          baseline: comparison.comparisons.errorRate.baseline,
          change: comparison.comparisons.errorRate.change,
          message: `Error rate increased by ${comparison.comparisons.errorRate.change.toFixed(2)}%`
        });
      }
    }

    // Check throughput regressions
    if (comparison.comparisons.throughput && comparison.comparisons.throughput.significant) {
      regressions.push({
        type: 'throughput',
        metric: 'overall',
        severity: this.calculateSeverity(
          Math.abs(comparison.comparisons.throughput.change),
          Math.abs(this.thresholds.throughput.threshold)
        ),
        current: comparison.comparisons.throughput.current,
        baseline: comparison.comparisons.throughput.baseline,
        change: comparison.comparisons.throughput.change,
        message: `Throughput decreased by ${Math.abs(comparison.comparisons.throughput.change).toFixed(2)}%`
      });
    }

    // Check endpoint-specific regressions
    if (comparison.comparisons.endpoints) {
      Object.entries(comparison.comparisons.endpoints).forEach(([endpoint, data]) => {
        if (data.responseTime) {
          Object.entries(data.responseTime).forEach(([percentile, rtData]) => {
            if (rtData.significant && rtData.changeType === 'increase') {
              regressions.push({
                type: 'endpointResponseTime',
                metric: `${endpoint}.${percentile}`,
                severity: this.calculateSeverity(rtData.change, this.thresholds.responseTime[percentile].threshold),
                current: rtData.current,
                baseline: rtData.baseline,
                change: rtData.change,
                message: `Endpoint ${endpoint} ${percentile} response time increased by ${rtData.change.toFixed(2)}%`
              });
            }
          });
        }
      });
    }

    console.log(`🎯 Found ${regressions.length} performance regressions`);
    return regressions;
  }

  calculateSeverity(change, threshold) {
    const ratio = Math.abs(change) / threshold;
    
    if (ratio >= 2) return 'critical';
    if (ratio >= 1.5) return 'high';
    if (ratio >= 1) return 'medium';
    return 'low';
  }

  generateReport(comparison, regressions) {
    console.log('📄 Generating performance report...');
    
    const report = {
      ...comparison,
      regressions: regressions,
      summary: {
        totalRegressions: regressions.length,
        criticalRegressions: regressions.filter(r => r.severity === 'critical').length,
        highRegressions: regressions.filter(r => r.severity === 'high').length,
        mediumRegressions: regressions.filter(r => r.severity === 'medium').length,
        lowRegressions: regressions.filter(r => r.severity === 'low').length,
        passed: regressions.length === 0
      }
    };

    // Save JSON report
    fs.writeFileSync(this.outputFile, JSON.stringify(report, null, 2));
    console.log(`💾 Performance report saved to ${this.outputFile}`);

    // Generate human-readable report
    this.generateHumanReadableReport(report);
    
    // Print summary to console
    this.printSummary(report);
  }

  generateHumanReadableReport(report) {
    const mdReportPath = this.outputFile.replace('.json', '.md');
    
    let content = '# Performance Comparison Report\n\n';
    content += `**Generated:** ${report.timestamp}\n\n`;
    content += `**Baseline:** ${report.baseline.timestamp}\n\n`;
    
    // Summary
    content += '## Summary\n\n';
    content += `- **Total Regressions:** ${report.summary.totalRegressions}\n`;
    content += `- **Critical:** ${report.summary.criticalRegressions}\n`;
    content += `- **High:** ${report.summary.highRegressions}\n`;
    content += `- **Medium:** ${report.summary.mediumRegressions}\n`;
    content += `- **Low:** ${report.summary.lowRegressions}\n`;
    content += `- **Status:** ${report.summary.passed ? '✅ PASSED' : '❌ FAILED'}\n\n`;

    // Regressions
    if (report.regressions.length > 0) {
      content += '## Performance Regressions\n\n';
      
      report.regressions.forEach((regression, index) => {
        const severityEmoji = {
          critical: '🔴',
          high: '🟠', 
          medium: '🟡',
          low: '🟢'
        };
        
        content += `### ${index + 1}. ${severityEmoji[regression.severity]} ${regression.message}\n\n`;
        content += `- **Type:** ${regression.type}\n`;
        content += `- **Metric:** ${regression.metric}\n`;
        content += `- **Severity:** ${regression.severity}\n`;
        content += `- **Current:** ${regression.current}\n`;
        content += `- **Baseline:** ${regression.baseline}\n`;
        content += `- **Change:** ${regression.change > 0 ? '+' : ''}${regression.change.toFixed(2)}%\n\n`;
      });
    }

    // Detailed comparisons
    if (report.comparisons.responseTime) {
      content += '## Response Time Comparison\n\n';
      content += '| Percentile | Current (ms) | Baseline (ms) | Change (%) | Status |\n';
      content += '|------------|--------------|---------------|------------|--------|\n';
      
      Object.entries(report.comparisons.responseTime).forEach(([percentile, data]) => {
        const status = data.significant ? 
          (data.changeType === 'increase' ? '❌' : '✅') : '✅';
        const change = data.change > 0 ? `+${data.change.toFixed(2)}` : data.change.toFixed(2);
        
        content += `| ${percentile} | ${data.current} | ${data.baseline} | ${change}% | ${status} |\n`;
      });
      content += '\n';
    }

    fs.writeFileSync(mdReportPath, content);
    console.log(`📄 Human-readable report saved to ${mdReportPath}`);
  }

  printSummary(report) {
    console.log('\n' + '='.repeat(60));
    console.log('📊 PERFORMANCE COMPARISON SUMMARY');
    console.log('='.repeat(60));
    
    if (report.summary.passed) {
      console.log('✅ Overall Status: PASSED');
    } else {
      console.log('❌ Overall Status: FAILED');
    }
    
    console.log(`📈 Total Regressions: ${report.summary.totalRegressions}`);
    
    if (report.summary.criticalRegressions > 0) {
      console.log(`🔴 Critical: ${report.summary.criticalRegressions}`);
    }
    if (report.summary.highRegressions > 0) {
      console.log(`🟠 High: ${report.summary.highRegressions}`);
    }
    if (report.summary.mediumRegressions > 0) {
      console.log(`🟡 Medium: ${report.summary.mediumRegressions}`);
    }
    if (report.summary.lowRegressions > 0) {
      console.log(`🟢 Low: ${report.summary.lowRegressions}`);
    }
    
    console.log('='.repeat(60));
  }
}

// CLI handling
if (require.main === module) {
  const args = process.argv.slice(2);
  const options = {};
  
  for (let i = 0; i < args.length; i += 2) {
    const key = args[i].replace('--', '');
    const value = args[i + 1];
    options[key] = value;
  }
  
  const comparator = new PerformanceComparator(options);
  comparator.compare();
}

module.exports = PerformanceComparator;