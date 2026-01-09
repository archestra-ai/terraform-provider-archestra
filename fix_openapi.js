const fs = require('fs');
const path = require('path');

const openapiPath = process.argv[2];

if (!openapiPath) {
    console.error('Usage: node fix_openapi.js <path_to_openapi.json>');
    process.exit(1);
}

const openapiContent = fs.readFileSync(openapiPath, 'utf8');
let openapi = JSON.parse(openapiContent);

function fixSchema(schema) {
    if (!schema || typeof schema !== 'object') {
        return schema;
    }

    // Check for anyOf with null
    if (schema.anyOf && Array.isArray(schema.anyOf) && schema.anyOf.length === 2) {
        const nullIndex = schema.anyOf.findIndex(s => s.type === 'null');
        if (nullIndex !== -1) {
            const otherIndex = nullIndex === 0 ? 1 : 0;
            const otherSchema = schema.anyOf[otherIndex];

            // Merge otherSchema into the parent, remove anyOf, add nullable: true
            const newSchema = { ...otherSchema, nullable: true };

            // Copy other properties from the original schema if they exist (e.g. description)
            for (const key in schema) {
                if (key !== 'anyOf') {
                    newSchema[key] = schema[key];
                }
            }
            return fixSchema(newSchema);
        }
    }

    // Recursively fix properties
    for (const key in schema) {
        schema[key] = fixSchema(schema[key]);
    }

    return schema;
}

openapi = fixSchema(openapi);

fs.writeFileSync(openapiPath, JSON.stringify(openapi, null, 2));
console.log('Fixed openapi.json');
